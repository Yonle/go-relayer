# toy: gwproxy

a random toy.

could be useful.

only using stdlib. concurrency is achieved via goroutine.

```
$ ./gwproxy
Usage of ./gwproxy:
  -dial-as string
        Dial to upstream with specified local IP address
  -from string
        Bind address
  -keepalive
        Enable KeepAlive (TCP)
  -keepalive-idle string
        Keep Alive idle duration (default "15s")
  -keepalive-interval string
        Keep Alive interval duration (default "15s")
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
