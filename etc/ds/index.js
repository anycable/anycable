#!/usr/bin/env node

/**
 * Durable Streams Test Client for AnyCable
 *
 * A beautiful CLI client for testing AnyCable's Durable Streams implementation.
 *
 * Usage: node index.js <stream-id> [options]
 *
 * Options:
 *   -u, --url <url>    Base URL for AnyCable server (default: http://localhost:8080)
 *   -p, --path <path>  DS endpoint path (default: /ds)
 *   -m, --mode <mode>  Read mode: catchup, poll, sse (default: catchup)
 *   -h, --help         Display help
 *   -V, --version      Display version
 */

import { stream } from "@durable-streams/client";
import { program } from "commander";
import chalk from "chalk";
import ora from "ora";
import boxen from "boxen";
import readline from "readline";

// ============================================================================
// CLI Setup
// ============================================================================

program
  .name("ds-client")
  .description("Durable Streams test client for AnyCable")
  .version("1.0.0")
  .argument("<stream-id>", "The stream ID to subscribe to")
  .option("-u, --url <url>", "Base URL for AnyCable server", process.env.DS_BASE_URL || "http://localhost:8080")
  .option("-p, --path <path>", "DS endpoint path", process.env.DS_PATH || "/ds")
  .option("-m, --mode <mode>", "Read mode: catchup, poll, sse", "catchup")
  .parse();

const options = program.opts();
const [streamId] = program.args;

// ============================================================================
// TUI Helpers
// ============================================================================

function printHeader(streamId, streamUrl, mode) {
  const header = boxen(
    [
      `${chalk.bold.cyan("🔌 AnyCable Durable Streams Client")}`,
      "",
      `${chalk.dim("Stream:")}  ${chalk.white(streamId)}`,
      `${chalk.dim("URL:")}     ${chalk.white(streamUrl)}`,
      `${chalk.dim("Mode:")}    ${chalk.yellow(mode)}`,
    ].join("\n"),
    {
      padding: 1,
      margin: { top: 1, bottom: 1 },
      borderStyle: "round",
      borderColor: "cyan",
    }
  );
  console.log(header);
}

function printMessage(index, message) {
  const content = typeof message === "string" ? message : JSON.stringify(message, null, 2);
  const box = boxen(content, {
    title: chalk.dim(`Message ${index}`),
    titleAlignment: "left",
    padding: { left: 1, right: 1, top: 0, bottom: 0 },
    borderStyle: "single",
    borderColor: "gray",
  });
  console.log(box);
}

function printSuccess(count, offset) {
  console.log(
    chalk.green(`✓ Received ${count} message${count !== 1 ? "s" : ""}`) +
      chalk.dim(` (offset: ${offset})`)
  );
  console.log();
}

function printNoMessages() {
  console.log(chalk.blue("ℹ No new messages (up to date)"));
  console.log();
}

function printOffsetUpdate(oldOffset, newOffset) {
  console.log(
    chalk.dim("↳ Offset: ") +
      chalk.gray(oldOffset) +
      chalk.dim(" → ") +
      chalk.white(newOffset)
  );
  console.log();
}

function printError(message) {
  console.log(chalk.red(`✗ Error: ${message}`));
  console.log();
}

function printNotImplemented(mode) {
  console.log(
    boxen(chalk.yellow(`⚠ ${mode.toUpperCase()} mode is not implemented yet.`), {
      padding: 1,
      borderStyle: "round",
      borderColor: "yellow",
    })
  );
}

function printPrompt() {
  process.stdout.write(chalk.cyan("▶ ") + chalk.dim("Press Enter to fetch more (Ctrl+C to exit)... "));
}

function printGoodbye() {
  console.log();
  console.log(chalk.dim("👋 Goodbye!"));
}

function waitForEnter() {
  return new Promise((resolve) => {
    printPrompt();

    let lineReceived = false;

    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
      terminal: false,
    });

    rl.once("line", () => {
      lineReceived = true;
      rl.close();
      resolve();
    });

    rl.once("close", () => {
      // Only treat as exit if we didn't receive a line (i.e., Ctrl+C)
      if (!lineReceived) {
        resolve("exit");
      }
    });
  });
};

// ============================================================================
// Stream Fetching
// ============================================================================

async function fetchMessages(streamUrl, offset, live = false) {
  const spinner = ora({
    text: "Fetching messages...",
    color: "cyan",
  }).start();

  try {
    const res = await stream({
      url: streamUrl,
      offset: offset,
      live,
    });

    const messages = await res.json();
    spinner.stop();

    return {
      messages,
      offset: res.offset,
      upToDate: res.upToDate,
    };
  } catch (error) {
    spinner.stop();
    throw error;
  }
}

// ============================================================================
// Mode Handlers
// ============================================================================

async function runCatchupMode(streamUrl, live = false) {
  let currentOffset = "-1";

  const poll = async () => {
    try {
      const result = await fetchMessages(streamUrl, currentOffset, live);

      if (result.messages.length > 0) {
        printSuccess(result.messages.length, result.offset);

        for (let i = 0; i < result.messages.length; i++) {
          printMessage(i + 1, result.messages[i]);
        }
      } else {
        printNoMessages();
      }

      if (result.offset && result.offset !== currentOffset) {
        printOffsetUpdate(currentOffset, result.offset);
        currentOffset = result.offset;
      }
    } catch (error) {
      printError(error.message);
    }
  };

  // Initial fetch
  await poll();

  // Main loop
  while (true) {
    const result = await waitForEnter();
    if (result === "exit") {
      printGoodbye();
      break;
    }
    // Clear the prompt line
    process.stdout.write("\r" + " ".repeat(60) + "\r");
    await poll();
  }
}

async function runPollMode(streamUrl) {
  await runCatchupMode(streamUrl, "long-poll");
  process.exit(0);
}

async function runSseMode(streamUrl) {
  let currentOffset = "-1";

  const spinner = ora({
    text: "Streaming messages... (hit ENTER to exit)",
    color: "cyan",
  }).start();

  try {
    const res = await stream({
      url: streamUrl,
      offset: currentOffset,
      live: "sse",
      json: true
    });

    let i = 1;

    res.subscribeJson(async (batch) => {
      console.log(batch);
      for (const item of batch.items) {
        printMessage(i++, item);
      }
    })

    await waitForEnter();
    printGoodbye();
    spinner.stop();

    process.exit(0);
  } catch (error) {
    spinner.stop();
    throw error;
  }
}

// ============================================================================
// Main
// ============================================================================

async function main() {
  const { url: baseUrl, path: dsPath, mode } = options;

  // Validate mode
  const validModes = ["catchup", "poll", "sse"];
  if (!validModes.includes(mode)) {
    console.error(chalk.red(`Error: Invalid mode "${mode}". Must be one of: ${validModes.join(", ")}`));
    process.exit(1);
  }

  const streamUrl = `${baseUrl}${dsPath}/${encodeURIComponent(streamId)}`;

  // Print header
  printHeader(streamId, streamUrl, mode);

  // Run the appropriate mode
  switch (mode) {
    case "catchup":
      await runCatchupMode(streamUrl);
      break;
    case "poll":
      await runPollMode(streamUrl);
      break;
    case "sse":
      await runSseMode(streamUrl);
      break;
  }
}

// Handle uncaught errors
main().catch((error) => {
  console.error(chalk.red(`Fatal error: ${error.message}`));
  process.exit(1);
});
