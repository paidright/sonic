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
	"strings"
	"syscall"
	"time"

	kewpie "github.com/davidbanham/kewpie_go"
	"github.com/davidbanham/kewpie_go/types"
	"github.com/paidright/sonic/config"
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

var ErrWebhookServerFailed = fmt.Errorf("The upstream server failed when trying to send the start webhook.")
var ErrWebhookBadRequest = fmt.Errorf("The upstream server indicated the request was bad.")

func subscribe(ctx context.Context) error {
	running := false

	handler := cliHandler{
		handleFunc: func(task kewpie.Task) (bool, error) {
			running = true
			defer func() {
				running = false
			}()
			if err := sendWebhook("start", task); err == ErrWebhookServerFailed {
				log.Printf("ERROR webhook error will requeue for task %+v\n", task)
				return true, err
			} else if err == ErrWebhookBadRequest {
				log.Printf("INFO abort signal received for task %+v\n", task)
				return false, err
			} else if err != nil {
				log.Printf("ERROR dealing with start webhook will not requeue for task %+v\n", task)
				return false, err
			}

			if err := runProc(ctx, task.Body); err != nil {
				if err := sendWebhook("fail", task); err != nil {
					log.Printf("ERROR sending failure webhook for task %+v\n", task)
				}
				return config.RETRY, err
			}
			if err := sendWebhook("success", task); err != nil {
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

func runProc(ctx context.Context, cli string) error {
	parts := strings.Split(cli, " ")
	command := parts[0]
	args := parts[1:]
	cmd := exec.CommandContext(ctx, command, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

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
func sendWebhook(event string, task kewpie.Task) error {
	tagName := "webhook_" + event
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
