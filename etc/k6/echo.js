// Build k6 with xk6-cable like this:
//    xk6 build v0.38.3 --with github.com/anycable/xk6-cable@v0.3.0

import { check, sleep, fail } from "k6";
import cable from "k6/x/cable";
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";
import { Trend } from "k6/metrics";

let commandTrend = new Trend("command_duration", true);

let config = __ENV;

let url = config.CABLE_URL || "ws://localhost:8080/cable";
let channelName = (config.CHANNEL_ID || 'BenchmarkChannel');

export const options = {
  scenarios: {
    default: {
      executor: 'externally-controlled',
      vus: 30,
      maxVUs: 500,
      duration: '5m'
    }
  }
};

export default function () {
  let cableOptions = {
    receiveTimeoutMs: 15000,
    headers: {}
  };

  let client

  try {
    client = cable.connect(url, cableOptions);

    if (
      !check(client, {
        "successful connection": (obj) => obj,
      })
    ) {
      fail("connection failed");
    }
  } catch (err) {
    console.error(err)
    return
  }

  let channel

  try {
    channel = client.subscribe(channelName);

    if (
      !check(channel, {
        "successful subscription": (obj) => obj,
      })
    ) {
      fail("failed to subscribe");
    }
  } catch (err) {
    console.error(err)
    return
  }

  for (let i = 0; i < 10; i++) {
    let start = Date.now();
    channel.perform("echo", { ts: start, content: `hello from ${__VU} numero ${i+1}` });

    sleep(randomIntBetween(5, 10) / 100);

    let incoming = channel.receiveAll(1);

    for(let message of incoming) {
      let received = message.__timestamp__ || Date.now();

      if (message.action == "echo") {
        let ts = message.ts;
        commandTrend.add(received - ts);
      }
    }

    sleep(randomIntBetween(5, 10) / 100);
  }

  sleep(randomIntBetween(2, 5));

  client.disconnect();
}
