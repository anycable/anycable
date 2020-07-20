# OS Tuning

## Open files limit

The most important thing you should take into account is to set a big enough open files limit.
It defines how many file descriptors a process can keep open, and a socket is also a file descriptor.
Thus, you cannot handle more connections than this limit (and event less, since the process uses a few file descriptors for its own purposes).

 AnyCable-Go prints the current open files limit on boot:

```sh
$ anycable-go
INFO 2020-06-07T19:30:33.059Z context=main Starting AnyCable v1.0.0s (with mruby 1.2.0 (2015-11-17)) (pid: 29333, open file limit: 524288)
...
```

Alternatively, you can run `ulimit -n` for the user which runs `anycable-go` or check the running process limits by `cat /proc/<process id>/limits`.

Changing this limit depends on the OS and the way you deploy the server (e.g., for [systemd](../deployment/systemd.md) you can set a limit using `LimitNOFILE` directive).

## TCP keepalive

WebSockets are implemented on top of the TCP protocol. Normally, closing a connection is happening via 4-step handshake. But what happens if there is no more network to send handshake packets? How TCP detects that connection was lost if there is no network? By using [_keepalive_](http://tldp.org/HOWTO/TCP-Keepalive-HOWTO/overview.html) feature.

Keepalive works the following way: if no data has been transmitting via socket for X seconds, the server sends N probe packets every Y seconds, and only if all of these packets failed to deliver, the server closes the socket. Thus, the `CLOSE` event could happen in minutes or even hours (depending on the OS settings) after the network failure.

The recommended configuration is the following (add it to `/etc/sysctl.conf`):

```sysctl
net.ipv4.tcp_keepalive_intvl = 10
net.ipv4.tcp_keepalive_probes = 5
net.ipv4.tcp_keepalive_time = 300
```

The configuration above will allow you to “catch” dead connection in ~6min.

**NOTE**: Don’t forget to reload the configuration by running sudo `sysctl -p /etc/sysctl.conf`.

## Resources

We recommend to check out these articles on the details of how to tune OS settings for _zillions_ of connections:

- [The Road to 2 Million Websocket Connections in Phoenix](https://phoenixframework.org/blog/the-road-to-2-million-websocket-connections)
- [Benchmarking and Scaling WebSockets: Handling 60000 concurrent connections](http://kemalcr.com/blog/2016/11/13/benchmarking-and-scaling-websockets-handling-60000-concurrent-connections/)
