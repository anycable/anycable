// Build k6 with xk6-cable like this:
//    xk6 build v0.38.3 --with github.com/anycable/xk6-cable@v0.3.0

import { check, sleep, fail } from "k6";
import cable from "k6/x/cable";
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";

const rampingOptions = {
  scenarios: {
    default: {
      executor: 'ramping-vus',
      startVUs: 100,
      stages: [
        { duration: '20s', target: 300 },
        { duration: '20s', target: 500 },
        { duration: '30s', target: 1000 },
        { duration: '30s', target: 1400 },
        { duration: '30s', target: 1800 },
        { duration: '50s', target: 2500 },
        { duration: '60s', target: 2500 },
        { duration: '120s', target: 0 },
      ],
      gracefulStop: '5m',
      gracefulRampDown: '5m',
    }
  }
};

export const options = __ENV.SKIP_OPTIONS ? {} : rampingOptions;

import { Trend, Counter } from "k6/metrics";
let rttTrend = new Trend("rtt", true);
let broadcastTrend = new Trend("broadcast_duration", true);
let broadcastsSent = new Counter("broadcasts_sent");
let broadcastsRcvd = new Counter("broadcasts_rcvd");
let acksRcvd = new Counter("acks_rcvd");

// Load ENV from .env
function loadDotEnv() {
  try {
    let dotenv = open("./.env")
    dotenv.split(/[\n\r]/m).forEach( (line) => {
      // Ignore comments
      if (line[0] === "#") return

      let parts = line.split("=", 2)

      __ENV[parts[0]] = parts[1]
    })
  } catch(_err) {
  }
}

loadDotEnv()

let config = __ENV

config.URL = config.URL || "ws://localhost:8080/cable";

let url = config.URL;
let channelName = (config.CHANNEL_ID || 'BenchmarkChannel');

let sendersRatio = parseFloat((config.SENDERS_RATIO || '0.2')) || 1;
let sendersMod = (1 / sendersRatio) | 0;
let sender = __VU % sendersMod == 0;

let sendingRate = parseFloat(config.SENDING_RATE || '0.2');

let iterations = (config.N || '100') | 0;

export default function () {
  let cableOptions = {
    receiveTimeoutMs: 15000
  }

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

  let channel = client.subscribe(channelName);

  if (
    !check(channel, {
      "successful subscription": (obj) => obj,
    })
  ) {
    fail("failed to subscribe");
  }

  for(let i = 0; ; i++) {
    // Sampling
    if (sender && (randomIntBetween(1, 10) / 10) <= sendingRate) {
      let start = Date.now();
      broadcastsSent.add(1);
      // Create message via cable instead of a form
      channel.perform("broadcast", { ts: start, content: `hello from ${__VU} numero ${i+1}` });
    }

    sleep(randomIntBetween(5, 10) / 100);

    let incoming = channel.receiveAll(1);

    for(let message of incoming) {
      let received = message.__timestamp__ || Date.now();

      if (message.action == "broadcastResult") {
        acksRcvd.add(1);
        let ts = message.ts;
        rttTrend.add(received - ts);
      }

      if (message.action == "broadcast") {
        broadcastsRcvd.add(1);
        let ts = message.ts;
        broadcastTrend.add(received - ts);
      }
    }

    sleep(randomIntBetween(5, 10) / 100);

    if (i > iterations) break;
  }

  client.disconnect();
}
