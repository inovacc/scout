#!/usr/bin/env node
"use strict";

const { execSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");
const { createWriteStream } = require("fs");

const REPO = "inovacc/scout";
const BIN_DIR = path.join(__dirname, "bin");

function getPlatform() {
  const platform = os.platform();
  const arch = os.arch();

  const platformMap = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows",
  };

  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const goOS = platformMap[platform];
  const goArch = archMap[arch];

  if (!goOS || !goArch) {
    throw new Error(`Unsupported platform: ${platform}/${arch}`);
  }

  return { goOS, goArch, ext: platform === "win32" ? "zip" : "tar.gz" };
}

function getLatestVersion() {
  return new Promise((resolve, reject) => {
    const options = {
      hostname: "api.github.com",
      path: `/repos/${REPO}/releases/latest`,
      headers: { "User-Agent": "scout-browser-npm" },
    };

    https
      .get(options, (res) => {
        let data = "";
        res.on("data", (chunk) => (data += chunk));
        res.on("end", () => {
          try {
            const json = JSON.parse(data);
            resolve(json.tag_name);
          } catch (e) {
            reject(new Error(`Failed to parse GitHub response: ${e.message}`));
          }
        });
      })
      .on("error", reject);
  });
}

function download(url, dest) {
  const MAX_REDIRECTS = 5;
  return new Promise((resolve, reject) => {
    const follow = (url, depth) => {
      if (depth > MAX_REDIRECTS) {
        reject(new Error(`Too many redirects (>${MAX_REDIRECTS})`));
        return;
      }
      https
        .get(url, { headers: { "User-Agent": "scout-browser-npm" } }, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            follow(res.headers.location, depth + 1);
            return;
          }
          if (res.statusCode !== 200) {
            reject(new Error(`Download failed: HTTP ${res.statusCode}`));
            return;
          }
          const file = createWriteStream(dest);
          res.pipe(file);
          file.on("finish", () => {
            file.close();
            resolve();
          });
        })
        .on("error", reject);
    };
    follow(url, 0);
  });
}

function extract(archive, dest, ext) {
  if (ext === "zip") {
    execSync(
      `powershell -Command "Expand-Archive -Path '${archive}' -DestinationPath '${dest}' -Force"`,
      { stdio: "pipe" }
    );
  } else {
    execSync(`tar -xzf "${archive}" -C "${dest}"`, { stdio: "pipe" });
  }
}

async function main() {
  const binaryName = os.platform() === "win32" ? "scout.exe" : "scout";
  const binaryPath = path.join(BIN_DIR, binaryName);

  if (fs.existsSync(binaryPath)) {
    console.log("scout: binary already installed");
    return;
  }

  const { goOS, goArch, ext } = getPlatform();

  console.log(`scout: detecting platform... ${goOS}/${goArch}`);

  let tag;
  try {
    tag = await getLatestVersion();
  } catch (e) {
    console.error(`scout: failed to get latest version: ${e.message}`);
    console.error("  You can install manually: go install github.com/inovacc/scout/cmd/scout@latest");
    process.exit(0);
  }

  const version = tag.replace(/^v/, "");
  const archive = `scout_${version}_${goOS}_${goArch}.${ext}`;
  const url = `https://github.com/${REPO}/releases/download/${tag}/${archive}`;

  console.log(`scout: downloading ${tag} for ${goOS}/${goArch}...`);

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "scout-"));
  const archivePath = path.join(tmpDir, archive);
  const extractDir = path.join(tmpDir, "extracted");

  try {
    await download(url, archivePath);

    // Verify SHA256 checksum from checksums.txt
    const checksumsUrl = `https://github.com/${REPO}/releases/download/${tag}/checksums.txt`;
    const checksumsPath = path.join(tmpDir, "checksums.txt");
    try {
      await download(checksumsUrl, checksumsPath);
      const checksums = fs.readFileSync(checksumsPath, "utf8");
      const line = checksums.split("\n").find((l) => l.includes(archive));
      if (line) {
        const expectedHash = line.trim().split(/\s+/)[0];
        const fileHash = crypto
          .createHash("sha256")
          .update(fs.readFileSync(archivePath))
          .digest("hex");
        if (fileHash !== expectedHash) {
          throw new Error(`checksum mismatch: expected ${expectedHash}, got ${fileHash}`);
        }
        console.log("scout: checksum verified");
      }
    } catch (checksumErr) {
      if (checksumErr.message.includes("checksum mismatch")) throw checksumErr;
      console.warn("scout: checksum verification skipped (checksums.txt unavailable)");
    }

    fs.mkdirSync(extractDir, { recursive: true });
    extract(archivePath, extractDir, ext);

    fs.mkdirSync(BIN_DIR, { recursive: true });

    const findBinary = (dir) => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const fullPath = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          const found = findBinary(fullPath);
          if (found) return found;
        } else if (entry.name === "scout" || entry.name === "scout.exe") {
          return fullPath;
        }
      }
      return null;
    };

    const binary = findBinary(extractDir);
    if (!binary) {
      throw new Error("scout binary not found in archive");
    }

    fs.copyFileSync(binary, binaryPath);
    fs.chmodSync(binaryPath, 0o755);

    console.log(`scout: installed ${tag} to ${binaryPath}`);
  } catch (e) {
    console.error(`scout: installation failed: ${e.message}`);
    console.error("  You can install manually: go install github.com/inovacc/scout/cmd/scout@latest");
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

main();
