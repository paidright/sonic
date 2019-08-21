package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	kewpie "github.com/davidbanham/kewpie_go"
	"github.com/davidbanham/kewpie_go/types"
	"github.com/paidright/sonic/config"
)

// Webhook is a callback Sonic uses to inform the creator of the
// original message what the status of the asynchronous command is
type Webhook int

const (
	_ = iota // Ignore first value
	startWebhook
	successWebhook
	failWebhook
)

var queue kewpie.Kewpie

func init() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(currentVersion)
		os.Exit(0)
	}
	queue.Connect(config.KEWPIE_BACKEND, []string{config.QUEUE})

	fmt.Printf("INFO listening on queue: %s \n", config.QUEUE)
}

type cliHandler struct {
	handleFunc func(types.Task) (bool, error)
}

func (h cliHandler) Handle(t types.Task) (bool, error) {
	return h.handleFunc(t)
}

func main() {
	ctx := contextWithSigterm(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				queue.Disconnect()
				return
			}
		}
	}()

	if err := subscribe(ctx); err != nil {
		log.Fatal("ERROR", err)
	}
}

// ErrWebhookServerFailed is returned as the catch all error on a callback.
var ErrWebhookServerFailed = fmt.Errorf("The upstream server failed when trying to send the start webhook")

// ErrWebhookBadRequest is returned when sonic issues a callback which returns an Http 400 code
var ErrWebhookBadRequest = fmt.Errorf("The upstream server indicated the request was bad")

// ErrUnknownWebhook is returned when a user specifies an event unknown to Kewpie
var ErrUnknownWebhook = fmt.Errorf("Unknown web hook")

/*
 * Subscribe to messages from the corresponding Kewpie queue. Initially signal that the requested
 * task has "started" meaning Sonic is ready to call the requested process. Sonic then calls the
 * process, if this fails it signals a fail via the webhook. If the process completes with a graceful
 * exit code, then Sonic signals a success via the webhook.
 */
func subscribe(ctx context.Context) error {
	running := false

	handler := cliHandler{
		handleFunc: func(task kewpie.Task) (bool, error) {
			running = true
			defer func() {
				running = false
			}()

			// Signal start
			if requeue, err := signalTaskStart(task); err != nil {
				return requeue, err
			}

			// Run proc, signal fail if it does fail
			if err := runProc(ctx, task.Body); err != nil {
				if err := sendWebhook(failWebhook, task); err != nil {
					log.Printf("ERROR sending failure webhook for task %+v\n", task)
				}
				return config.RETRY, err
			}

			// Signal success/complete
			if err := sendWebhook(successWebhook, task); err != nil {
				log.Printf("ERROR sending success webhook for task %+v\n", task)
			}

			return false, nil
		},
	}

	if config.DIE_IF_IDLE {
		go func() {
			for {
				time.Sleep(config.MAX_IDLE)
				if !running {
					os.Exit(0)
				}
			}
		}()
	}

	if config.SINGLE_SHOT {
		return queue.Pop(ctx, config.QUEUE, handler)
	}
	return queue.Subscribe(ctx, config.QUEUE, handler)
}

/*
 * Run a command in the container. Output is piped to
 * stdout, and errors to stderr.
 */
func runProc(ctx context.Context, cli string) error {
	command, args := getCommandAndArgs(cli)
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

/*
 * Signal that the task is about to commence. The bool tells Kewpie whether the
 * task needs to be requeued
 */
func signalTaskStart(task kewpie.Task) (bool, error) {
	if err := sendWebhook(startWebhook, task); err == ErrWebhookServerFailed {
		log.Printf("ERROR webhook error will requeue for task %+v\n", task)
		return true, err
	} else if err == ErrWebhookBadRequest {
		log.Printf("INFO abort signal received for task %+v\n", task)
		return false, err
	} else if err != nil {
		log.Printf("ERROR dealing with start webhook will not requeue for task %+v\n", task)
		return false, err
	}
	return false, nil
}

/*
 * Load command and arguments from the cli text. Golang is very forgiving
 * when it parses the string, even handling empty strings!
 */
func getCommandAndArgs(cli string) (string, []string) {
	regXp := regexp.MustCompile(`\s+`)
	parts := regXp.Split(cli, -1)
	command := parts[0]
	args := parts[1:]

	return command, args
}

/*
 * Creates a derived (child) context using the parent context. The derived
 * context is a WithCancel context which prevents the go routine from leaking.
 * Cancel is deferred and called witht the go routine.
 */
func contextWithSigterm(ctx context.Context) context.Context {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()

		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		select {
		case <-signalCh:
		case <-ctx.Done():
		}
	}()

	return ctxWithCancel
}

/*
 * When kewpie pulls a message of a queue, it communicates the progress
 * of Sonic's execution via 3 webhooks, start, fail and success which
 * issues a HTTP post to an end point defined in the task.Tags map.
 */
func sendWebhook(event Webhook, task kewpie.Task) error {
	evt, err := webhookToString(event)
	if err != nil {
		return err
	}

	tagName := "webhook_" + evt
	if task.Tags[tagName] == "" {
		return nil
	}

	payload, err := json.Marshal(task)
	if err != nil {
		log.Printf("Error marshalling JSON %+v\n", err)
		return err
	}

	log.Printf("INFO Sending a http post for event %+v on the url %+v\n", tagName, task.Tags[tagName])
	res, err := http.Post(task.Tags[tagName], "application/json", bytes.NewReader(payload))

	if err != nil {
		log.Printf("ERROR webhook error %+v\n", err)
		return ErrWebhookServerFailed
	}

	log.Printf("INFO Response code from post %+v\n", res.StatusCode)
	if res.StatusCode == 400 {
		return ErrWebhookBadRequest
	}

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	return ErrWebhookServerFailed
}

/*
 * We represent Webhooks a using integers to make the code a bit safer. golang is a bit
 * loose with it's enums.
 */
func webhookToString(hook Webhook) (string, error) {
	switch hook {
	case 1:
		return "start", nil
	case 2:
		return "success", nil
	case 3:
		return "fail", nil
	default:
		return "", ErrUnknownWebhook
	}
}
