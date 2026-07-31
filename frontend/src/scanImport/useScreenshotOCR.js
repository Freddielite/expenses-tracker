import { useCallback, useRef, useState } from "react";
import { createWorker } from "tesseract.js";

// Wraps tesseract.js so the rest of the app doesn't need to know anything
// about workers/progress events. Everything runs client-side in the
// browser — the image never leaves the device, no API key, no cost.
//
// IMPORTANT: by default tesseract.js fetches its worker script, wasm core,
// and language training data from the jsdelivr CDN at runtime. That's fine
// on a normal internet connection, but this app is often tested over a bare
// LAN IP with no general internet access — in that situation the CDN fetch
// just hangs (no error, no progress), which looks exactly like a frozen
// "Reading the screenshot…" spinner. So every asset is pointed at a local
// copy under /tesseract instead (see public/tesseract/), and a timeout
// below turns a genuine network stall into a visible error rather than an
// infinite spinner.
const BASE = `${import.meta.env.BASE_URL}tesseract/`;
const WORKER_PATH = `${BASE}worker.min.js`;
// Pointing at a specific .js file (rather than a directory) skips
// tesseract.js's runtime SIMD/relaxed-SIMD feature detection, which
// otherwise picks a core variant based on what the browser supports and
// can request a file we never bundled locally (404 inside the worker,
// surfacing as an opaque "couldn't read text" error). SIMD has been
// standard in every mobile/desktop browser since 2021, so this is safe.
const CORE_PATH = `${BASE}core/tesseract-core-simd-lstm.wasm.js`;
const LANG_PATH = `${BASE}lang-data/`;

const LOAD_TIMEOUT_MS = 20000;

function withTimeout(promise, ms, message) {
  return Promise.race([
    promise,
    new Promise((_, reject) => setTimeout(() => reject(new Error(message)), ms)),
  ]);
}

export function useScreenshotOCR() {
  const [status, setStatus] = useState("idle"); // idle | recognizing | done | error
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState(null);
  const workerRef = useRef(null);

  const recognize = useCallback(async (file) => {
    setStatus("recognizing");
    setProgress(0);
    setError(null);
    let worker;
    try {
      // A fresh worker per scan is simplest and avoids stale-state bugs;
      // OCR on one screenshot is a one-off action, not a hot path.
      worker = await withTimeout(
        createWorker("eng", 1, {
          workerPath: WORKER_PATH,
          corePath: CORE_PATH,
          langPath: LANG_PATH,
          gzip: true,
          logger: (m) => {
            if (m.status === "recognizing text" && typeof m.progress === "number") {
              setProgress(m.progress);
            }
          },
        }),
        LOAD_TIMEOUT_MS,
        "Couldn't load the text-recognition engine. Check your connection and try again."
      );
      workerRef.current = worker;

      const { data } = await withTimeout(
        worker.recognize(file),
        LOAD_TIMEOUT_MS,
        "Reading the screenshot is taking too long. Try a smaller or clearer image."
      );

      setStatus("done");
      setProgress(1);
      return data.text || "";
    } catch (err) {
      // eslint-disable-next-line no-console
      console.error("Screenshot OCR failed:", err);
      setStatus("error");
      setError(err.message || "Couldn't read text from that image.");
      throw err;
    } finally {
      if (worker) {
        worker.terminate().catch(() => {});
      }
      workerRef.current = null;
    }
  }, []);

  const reset = useCallback(() => {
    setStatus("idle");
    setProgress(0);
    setError(null);
  }, []);

  return { recognize, reset, status, progress, error };
}
