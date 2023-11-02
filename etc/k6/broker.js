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
        { duration: "10s", target: 500 },
        { duration: "10s", target: 1000 },
        { duration: "10s", target: 1300 },
        { duration: "10s", target: 1500 },
        { duration: "30s", target: 1500 },
        { duration: "30s", target: 0 },
      ],
      gracefulStop: "1m",
      gracefulRampDown: "1m",
    },
  },
};

export const options = __ENV.SKIP_OPTIONS ? {} : rampingOptions;

import { Trend, Counter } from "k6/metrics";
let rttTrend = new Trend("rtt", true);
let subTrend = new Trend("suback", true);
let broadcastTrend = new Trend("broadcast_duration", true);
let historyTrend = new Trend("history_duration", true);
let historyRcvd = new Counter("history_rcvd");
let broadcastsSent = new Counter("broadcasts_sent");
let broadcastsRcvd = new Counter("broadcasts_rcvd");
let acksRcvd = new Counter("acks_rcvd");

// Load ENV from .env
function loadDotEnv() {
  try {
    let dotenv = open("./.env");
    dotenv.split(/[\n\r]/m).forEach((line) => {
      // Ignore comments
      if (line[0] === "#") return;

      let parts = line.split("=", 2);

      __ENV[parts[0]] = parts[1];
    });
  } catch (_err) {}
}

loadDotEnv();

let config = __ENV;

config.URL = config.URL || "ws://localhost:8080/cable";

let url = config.URL;
let channelName = config.CHANNEL_ID || "BenchmarkChannel";

let numChannels = parseInt(config.NUM_CHANNELS || "5") || 5;
let channelStreamId = __VU % numChannels;

let sendersRatio = parseFloat(config.SENDERS_RATIO || "0.3") || 1;
let sendersMod = (1 / sendersRatio) | 0;
let sender = __VU % sendersMod == 0;

let sendingRate = parseFloat(config.SENDING_RATE || "0.3");

let iterations = (config.N || "20") | 0;

export default function () {
  let cableOptions = {
    receiveTimeoutMs: 15000,
    logLevel: config.DEBUG === "true" ? "debug" : "info",
  };

  // Prevent from creating a lot of connections at once
  sleep(randomIntBetween(2, 10) / 5);

  let client = cable.connect(url, cableOptions);

  if (
    !check(client, {
      "successful connection": (obj) => obj,
    })
  ) {
    // Cooldown
    sleep(randomIntBetween(5, 10) / 5);
    fail("connection failed");
  }

  let channel = client.subscribe(channelName, { id: channelStreamId });

  if (
    !check(channel, {
      "successful subscription": (obj) => obj,
    })
  ) {
    // Cooldown
    sleep(randomIntBetween(5, 10) / 5);
    fail("failed to subscribe");
  }

  subTrend.add(channel.ackDuration());

  let subscribedAt = Date.now();

  let result = channel.history({ since: ((Date.now() - 10000) / 1000) | 0 });
  check(result, {
    "history received": (obj) => obj,
  });
  historyTrend.add(channel.historyDuration());

  for (let i = 0; ; i++) {
    // Sampling
    if (sender && randomIntBetween(1, 10) / 10 <= sendingRate) {
      let start = Date.now();
      broadcastsSent.add(1);
      // Create message via cable instead of a form
      channel.perform("broadcast", {
        ts: start,
        content: `hello from ${__VU} numero ${i + 1}`,
      });
    }

    sleep(randomIntBetween(5, 10) / 100);

    let incoming = channel.receiveAll(1);

    for (let message of incoming) {
      let received = message.__timestamp__;

      if (message.action == "broadcastResult") {
        acksRcvd.add(1);
        let ts = message.ts;
        rttTrend.add(received - ts);
      }

      if (message.action == "broadcast") {
        let ts = message.ts;

        // Historical message, shouldn't be taken into account for broadcast duration
        if (ts < subscribedAt) {
          historyRcvd.add(1);
          continue;
        }

        broadcastsRcvd.add(1);
        broadcastTrend.add(received - ts);
      }
    }

    sleep(randomIntBetween(5, 10) / 10);

    if (i > iterations) break;
  }

  sleep(randomIntBetween(5, 10) / 10);

  client.disconnect();
}
