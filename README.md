# toy: gwproxy

a random toy.

could be useful.

100% only using stdlib. concurrency is achieved via goroutine.

```
Usage of ./gwproxy:
  -bind string
    	Dial to upstream with specified local IP address (Bind)
  -clientbuffersize int
    	Client buffer size in bytes (default 4096)
  -from string
    	Listen to address
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
    	Destination upstream address
  -upstreambuffersize int
    	Upstream buffer size in bytes (default 4096)
```

## compiling

you will need atleast go 1.24.2 installed in your system.

```
go build -o gwproxy
```
