## Sonic

> It's a runner!

![Sonic Logo](https://notbad.software/img/sonic_logo.gif "Animated gif of Sonic the Hedgehog running")

Sonic plucks jobs from a [Kewpie](https://github.com/davidbanham/kewpie_go) queue and runs their body on the command line. The body of the task should be the string you would type in as you were sitting at the terminal, ie:

```
{
  "body": "echo 'Sonic is rad\!'",
}
```

It's handy for when you want to drive a workload from a Kewpie task queue, but don't want to integrate the queue consumption library into the language the service is written in.

### Running it

You must declare the queue you wish to listen to, the kewpie backend to use, whether to retry failed jobs, and any vars required by your chosen backend. eg:

```
export DB_URI=postgres://kewpie:wut@localhost:5432/kewpie?sslmode=disable
export KEWPIE_BACKEND=postgres
export QUEUE=lolhai
export RETRY=true
export SINGLE_SHOT=false
export DIE_IF_IDLE=false
export MAX_IDLE=30s
```

`RETRY` controls whether or not a task that failed (exited > 0) will be retried
`SINGLE_SHOT` mode tells Sonic to exit its own process after handling its first task and not look for a second one
`DIE_IF_IDLE` tells Sonic to exit if it is ever idle for more than `MAX_IDLE`
`MAX_IDLE` is a Go style Duration string. If `DIE_IF_IDLE` is not set, this setting has no effect

### Using it

Sonic will check the Tags attribute of a Kewpie task for webhooks to call on start, success and error.

```
{
  "body": "echo 'Sonic is rad\!'",
  "tags": {
    "webhook_start": "http://example.com/telemetry/start",
    "webhook_success": "http://example.com/telemetry/success",
    "webhook_error": "http://example.com/telemetry/error",
  }
}
```

If these are present, Sonic will send a POST payload with the contents of the task.

For the start webhook, if the server returns a `400` error code Sonic will abort the task and not requeue it. If the server returns anything in the `2xx` range the task will be run. Any other response will be treated as an error and Sonic will abort this run of the task and requeue it to be retried.
