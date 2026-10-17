# #32 orchestrator progress
- [x] start: ydk task start 32 --base origin/main (worktree .ydk/worktrees/32)
- [x] plan: main.go -> app{stdout,addr,connect,shutdownTimeout,onReady}.run(ctx) int; Kafka boundary = kafkaSide{publisher,consumer,client} (kgo.Client satisfies). Order: SetReady(false) -> srv.Shutdown -> Flush -> stop consumer -> CommitMarkedOffsets -> LeaveGroupContext -> Close; one 10s deadline, WARN step + exit 1. httpapi.NewServer panics on nil Censor/Publisher/Store.
- [x] build (TDD): server_test.go + main_test.go red first; gofmt/vet/test -race green.
- [x] review r1 (cavecrew-reviewer sonnet): fixed Close bounded by deadline, deadline reported only on step error, real connectKafka no-broker test, deadline test asserts skipped steps, fmt.Sprint in recover check.
- [x] review r2: clean; widened no-broker bound to 3 s.
- [x] verify: gofmt clean, go vet ok, go test -race ./... ok; smoke binary (PLAINTEXT, broker 127.0.0.1:1): ready 45 ms, /healthz 200, /readyz 200, CTRL_BREAK -> shutdown complete, exit 0 in ~3 ms.
- [ ] commit + ydk task done
