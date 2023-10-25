#!/usr/bin/env node

import { spawnSync } from "child_process";
import path from "path";
import fs from "fs";
import { install } from "../install.mjs";
import { fileURLToPath } from "url";
const __dirname = path.dirname(fileURLToPath(import.meta.url));

const extension = ["win32", "cygwin"].includes(process.platform) ? ".exe" : "";
const exePath = path.join(__dirname, `anycable-go${extension}`);

// Check if binary exists and download if not
if (!fs.existsSync(exePath)) {
  console.log("Installing AnyCable Go binary...");
  await install();
  console.log("Installed!");
}

var command_args = process.argv.slice(2);

spawnSync(exePath, command_args, { stdio: "inherit" });
