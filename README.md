# toy: gwproxy

a random toy.

could be useful.

only using stdlib. concurrency is achieved via goroutine.

```
$ ./gwproxy
Usage of ./gwproxy:
  -from string
        Bind address
  -proto string
        Protocol to use (default "tcp")
  -timeout string
        Timeout duration for upstream dial (default "5s")
  -to string
        Proxy target
```

## compiling

you will need atleast go 1.24.2 installed in your system.

```
go build -o gwproxy
```
