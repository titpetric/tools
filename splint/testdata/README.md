# The fixture

Code that fails every check on purpose, so a linter has something to find and a
test has something to assert on.

It is a module of its own and lives under `testdata/`, which every parser skips
along with `vendor`, `node_modules` and anything opening on `.` or `_`. Nothing
reads it by accident; it is read by being pointed at:

```shell
splint -i testdata ./...
splint -stats -i testdata ./...
```

Every file says at the top which check it is here to fail. Nothing in here is
meant to be good code, and `go vet` has opinions about some of it that are
entirely correct.
