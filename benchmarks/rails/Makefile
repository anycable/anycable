websocket-bench:
	websocket-bench broadcast ws://localhost:8080/cable \
		--concurrent 8 \
		--sample-size 100 \
		--step-size 200 \
		--payload-padding 200 \
		--total-steps 10 \
		--origin http://0.0.0.0 \
		--server-type=actioncable

.PHONY: websocket-bench
