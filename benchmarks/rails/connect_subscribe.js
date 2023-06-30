// Build k6 with xk6-cable like this:
//    xk6 build v0.38.3 --with github.com/anycable/xk6-cable@v0.3.0

import { check, sleep, fail } from "k6";
import cable from "k6/x/cable";
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";

const rampingOptions = {
  scenarios: {
    default: {
      executor: "ramping-vus",
      startVUs: 100,
      stages: [
        { duration: "10s", target: 300 },
        { duration: "20s", target: 500 },
        { duration: "30s", target: 1000 },
        { duration: "60s", target: 1000 },
        { duration: "120s", target: 0 },
      ],
      gracefulStop: "5m",
      gracefulRampDown: "5m",
    },
  },
};

export const options = __ENV.SKIP_OPTIONS ? {} : rampingOptions;

let config = __ENV;

config.URL = config.URL || "ws://localhost:8080/cable";

let url = config.URL;
let channelName = config.CHANNEL || "BenchmarkChannel";
let streamsNum = (config.STREAMS || "50") | 0;
let streamId = __VU % streamsNum;
let iterations = (config.N || "100") | 0;

export default function () {
  let cableOptions = {
    receiveTimeoutMs: 15000,
  };

  // Prevent from creating a lot of connections at once
  sleep(randomIntBetween(5, 10) / 10);

  let client = cable.connect(url, cableOptions);

  if (
    !check(client, {
      "successful connection": (obj) => obj,
    })
  ) {
    fail("connection failed");
  }

  let channel = client.subscribe(channelName, { id: streamId });

  if (
    !check(channel, {
      "successful subscription": (obj) => obj,
    })
  ) {
    fail("failed to subscribe");
  }

  for (let i = 0; ; i++) {
    // Sampling
    if (randomIntBetween(1, 10) / 10 <= 0.3) {
      channel.perform("broadcast", {
        content: `hello from ${__VU} numero ${i + 1}`,
      });
    }

    sleep(randomIntBetween(5, 10) / 100);

    let incoming = channel.receiveAll(0.2);

    sleep(randomIntBetween(5, 10) / 100);

    if (i > iterations) break;
  }

  client.disconnect();
}
