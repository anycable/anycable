// Build k6 with xk6-cable like this:
//    xk6 build v0.38.3 --with github.com/anycable/xk6-cable@v0.3.0

import { check, sleep, fail } from "k6";
import http from 'k6/http';
import cable from "k6/x/cable";
import { randomIntBetween } from "./utils/index.js";

let durationScale = __ENV.DURATION_SCALE || "0";

const rampingOptions = {
  scenarios: {
    default: {
      executor: "ramping-vus",
      startVUs: 100,
      stages: [
        { duration: `1${durationScale}s`, target: 300 },
        { duration: `1${durationScale}s`, target: 500 },
        { duration: `1${durationScale}s`, target: 1000 },
        { duration: `1${durationScale}s`, target: 1300 },
        { duration: `1${durationScale}s`, target: 1500 },
        { duration: `3${durationScale}s`, target: 1500 },
        { duration: `3${durationScale}s`, target: 0 },
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
let broadcastUrl = config.BROADCAST_URL || "http://localhost:8090/_broadcast";
let channelName = config.CHANNEL_ID || "BenchmarkChannel";

let numChannels = parseInt(config.NUM_CHANNELS || "5") || 5;
let channelStreamId = __VU % numChannels;

let sendersRatio = parseFloat(config.SENDERS_RATIO || "0.3") || 1;
let sendersMod = (1 / sendersRatio) | 0;
let sender = __VU % sendersMod == 0;

let sendingRate = parseFloat(config.SENDING_RATE || "0.3");
let sendingViaPerformRate = parseFloat(config.SENDING_WS_RATE || "0.3");

let iterations = (config.N || "20") | 0;

let payloadScale = (config.PAYLOAD_SCALE || "1") | 0;

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

      let content = `hello from ${__VU} numero ${i + 1}`.repeat(randomIntBetween(1, payloadScale));

      if (randomIntBetween(1, 10) / 10 <= sendingViaPerformRate) {
        channel.perform("broadcast", {
          ts: start,
          content,
        });
      } else {
        // Create message via HTTP broadcast intead of perform
        http.post(broadcastUrl, JSON.stringify({
          stream: `all${channelStreamId}`,
          data: JSON.stringify({
            ts: start,
            content,
          })
        }));
      }
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
