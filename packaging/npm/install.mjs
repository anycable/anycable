import { spawnSync } from "child_process";
import { chmodSync } from "fs";

const iswin = ["win32", "cygwin"].includes(process.platform);

export async function install() {
  if (process.env.CI) {
    return;
  }
  const exePath = await downloadBinary();
  if (!iswin) {
    chmodSync(exePath, "755");
  }

  // verify installation
  spawnSync(exePath, ["-v"], {
    stdio: "inherit",
  });
}

const baseReleaseUrl =
  "https://github.com/anycable/anycable-go/releases/download/";

import packageJson from "./package.json" with { type: "json" };
const packageVersion = packageJson.version;
const version = packageVersion.replace(/-patch.*$/, "");

function getDownloadURL() {
  // Detect OS
  // https://nodejs.org/api/process.html#process_process_platform
  let downloadOS = process.platform;
  let extension = "";
  if (iswin) {
    downloadOS = "win";
    extension = ".exe";
  }

  // Detect architecture
  // https://nodejs.org/api/process.html#process_process_arch
  let arch = process.arch;

  // Based on https://github.com/anycable/anycable-rails/blob/master/lib/generators/anycable/with_os_helpers.rb#L20
  switch (process.arch) {
    case "x64": {
      arch = "amd64";
      break;
    }
  }

  return `${baseReleaseUrl}/v${version}/anycable-go-${downloadOS}-${arch}${extension}`;
}

import { DownloaderHelper } from "node-downloader-helper";
import * as path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

async function downloadBinary() {
  // TODO zip the binaries to reduce the download size
  const downloadURL = getDownloadURL();
  const extension = iswin ? ".exe" : "";
  const fileName = `anycable-go${extension}`;
  const binDir = path.join(__dirname, "bin");
  const dl = new DownloaderHelper(downloadURL, binDir, {
    fileName,
    retry: { maxRetries: 5, delay: 50 },
  });

  console.log("Downloading anycable-go binary from " + downloadURL + "...");
  dl.on("end", () => console.log("anycable-go binary was downloaded"));
  try {
    await dl.start();
  } catch (e) {
    const message = `Failed to download ${fileName}: ${e.message} while fetching ${downloadURL}`;
    console.error(message);
    throw new Error(message);
  }
  return path.join(binDir, fileName);
}
