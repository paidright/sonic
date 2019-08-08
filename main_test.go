package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	kewpie "github.com/davidbanham/kewpie_go"
	"github.com/paidright/sonic/config"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestRunProc(t *testing.T) {
	uniq := uuid.NewV4().String()
	path := "/tmp/" + uniq

	ctx, cancel := context.WithCancel(context.Background())

	assert.Nil(t, runProc(ctx, "touch "+path))
	_, err := os.Open(path)
	assert.Nil(t, err)
	assert.Nil(t, os.Remove(path))

	cancel()
}

func TestSubscribe(t *testing.T) {
	uniq := uuid.NewV4().String()
	path := "/tmp/" + uniq

	payload := kewpie.Task{
		Body: "touch " + path,
	}

	queue.Publish(context.Background(), config.QUEUE, &payload)

	ctx, cancel := context.WithCancel(context.Background())
	go subscribe(ctx)

	time.Sleep(10 * time.Millisecond)

	_, err := os.Open(path)
	assert.Nil(t, err)
	assert.Nil(t, os.Remove(path))

	cancel()
}

func TestWebhooks(t *testing.T) {
	uniq := uuid.NewV4().String()
	path := "/tmp/" + uniq

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	go (func() {
		assert.Nil(t, http.Serve(listener, nil))
	})()

	called := map[string]bool{}

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		called["start"] = true
		body := kewpie.Task{}
		payload, err := ioutil.ReadAll(r.Body)
		assert.Nil(t, err)
		assert.Nil(t, json.Unmarshal(payload, &body))
		assert.Equal(t, body.Body, "echo "+uniq)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/dont_start", func(w http.ResponseWriter, r *http.Request) {
		called["dont_start"] = true
		w.WriteHeader(http.StatusBadRequest)
	})

	http.HandleFunc("/success", func(w http.ResponseWriter, r *http.Request) {
		called["success"] = true
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/fail", func(w http.ResponseWriter, r *http.Request) {
		called["fail"] = true
		w.WriteHeader(http.StatusOK)
	})

	payload := kewpie.Task{
		Body: "echo " + uniq,
		Tags: kewpie.Tags{
			"webhook_start":   "http://localhost:" + port + "/start",
			"webhook_success": "http://localhost:" + port + "/success",
		},
	}

	payloadNoRun := kewpie.Task{
		Body: "touch " + path,
		Tags: kewpie.Tags{
			"webhook_start":   "http://localhost:" + port + "/dont_start",
			"webhook_success": "http://localhost:" + port + "/success",
		},
	}

	failPayload := kewpie.Task{
		Body: "exit 1",
		Tags: kewpie.Tags{
			"webhook_fail": "http://localhost:" + port + "/fail",
		},
	}

	queue.Publish(context.Background(), config.QUEUE, &payload)
	queue.Publish(context.Background(), config.QUEUE, &payloadNoRun)
	queue.Publish(context.Background(), config.QUEUE, &failPayload)

	ctx, cancel := context.WithCancel(context.Background())
	go subscribe(ctx)

	time.Sleep(10 * time.Millisecond)

	if _, err := os.Open(path); err == nil {
		t.Fatal("The task ran and it shouldn't have")
	}

	assert.True(t, called["start"])
	assert.True(t, called["success"])
	assert.True(t, called["fail"])
	assert.True(t, called["dont_start"])

	cancel()
}