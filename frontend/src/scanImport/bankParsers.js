// Turns raw OCR text from a screenshot of a bank/fintech app's transaction
// list into a set of draft transactions the user can review and edit before
// anything is saved. This is deliberately a *best effort* parser, not a
// guarantee — OCR misreads and every app tweaks its layout over time, so
// the review step in ScanImportModal is the real safety net, not this file.

const MONTHS = {
  jan: 0, feb: 1, mar: 2, apr: 3, may: 4, jun: 5,
  jul: 6, aug: 7, sep: 8, oct: 9, nov: 10, dec: 11,
};

const WEEKDAY_HEADER = /^(mon|tue|wed|thu|fri|sat|sun)[a-z]*\b/i;
// "14 Jul 2026" / "14th Jul" style — day before month.
const EXPLICIT_DATE = /\b(\d{1,2})\s*(?:st|nd|rd|th)?[\s,-]+(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?(?:[\s,-]+(\d{2,4}))?/i;
// "Jul 13th, 2026" style — month before day, as seen on single-transaction
// receipt/detail screens (e.g. "Transaction Date: Jul 13th, 2026 21:11:01").
const EXPLICIT_DATE_MONTH_FIRST =
  /\b(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.?\s+(\d{1,2})(?:st|nd|rd|th)?(?:[\s,-]+(\d{2,4}))?/i;

// Lines that mention these are reward/bonus metadata attached to a receipt,
// not a separate transaction — without this they can get misread as a
// second, phantom income entry sitting right next to the real one.
const NOISE_LINE = /\b(cashback|bonus earned|reward)\b/i;

// Matches a Naira amount with an optional leading sign, e.g. "-₦14,500.00",
// "+N20,000", "NGN 5,000.00", "5,000.00 Dr", "2,000 Cr".
const AMOUNT_RE =
  /([+-])?\s*(?:₦|N(?:GN)?)\s*([\d][\d,]*(?:\.\d{1,2})?)|([\d][\d,]*(?:\.\d{1,2})?)\s*(?:₦|N(?:GN)?)\s*(dr|cr)?\b|([\d][\d,]*(?:\.\d{1,2})?)\s+(dr|cr)\b/i;

function toISODate(year, monthIdx, day) {
  const d = new Date(Date.UTC(year, monthIdx, day));
  return d.toISOString().slice(0, 10);
}

// Resolves a date-header line ("Today", "Yesterday", "14 Jul", "Mon 14 Jul
// 2026") into an ISO date, relative to `now`. Returns null if the line
// doesn't look like a date header at all.
export function parseDateHeader(line, now = new Date()) {
  const trimmed = line.trim();
  if (/^today\b/i.test(trimmed)) {
    return now.toISOString().slice(0, 10);
  }
  if (/^yesterday\b/i.test(trimmed)) {
    const d = new Date(now);
    d.setUTCDate(d.getUTCDate() - 1);
    return d.toISOString().slice(0, 10);
  }

  const explicit = trimmed.match(EXPLICIT_DATE);
  if (explicit) {
    const day = parseInt(explicit[1], 10);
    const monthIdx = MONTHS[explicit[2].slice(0, 3).toLowerCase()];
    let year = explicit[3] ? parseInt(explicit[3], 10) : now.getUTCFullYear();
    if (year < 100) year += 2000;
    if (monthIdx === undefined || day < 1 || day > 31) return null;

    // No explicit year on screen — guard against screenshots straddling a
    // year boundary by picking whichever nearby year keeps the date from
    // landing more than ~2 days in the future.
    if (!explicit[3]) {
      const candidate = new Date(Date.UTC(year, monthIdx, day));
      const twoDaysFromNow = new Date(now);
      twoDaysFromNow.setUTCDate(twoDaysFromNow.getUTCDate() + 2);
      if (candidate > twoDaysFromNow) year -= 1;
    }
    return toISODate(year, monthIdx, day);
  }

  const monthFirst = trimmed.match(EXPLICIT_DATE_MONTH_FIRST);
  if (monthFirst) {
    const monthIdx = MONTHS[monthFirst[1].slice(0, 3).toLowerCase()];
    const day = parseInt(monthFirst[2], 10);
    let year = monthFirst[3] ? parseInt(monthFirst[3], 10) : now.getUTCFullYear();
    if (year < 100) year += 2000;
    if (monthIdx === undefined || day < 1 || day > 31) return null;

    if (!monthFirst[3]) {
      const candidate = new Date(Date.UTC(year, monthIdx, day));
      const twoDaysFromNow = new Date(now);
      twoDaysFromNow.setUTCDate(twoDaysFromNow.getUTCDate() + 2);
      if (candidate > twoDaysFromNow) year -= 1;
    }
    return toISODate(year, monthIdx, day);
  }

  // A bare weekday name ("Mon", "Wednesday") with nothing else useful —
  // treat as a header we recognize but can't resolve precisely; caller
  // keeps the previous known date instead of guessing further.
  if (WEEKDAY_HEADER.test(trimmed) && trimmed.length < 16) {
    return null;
  }

  return null;
}

// True if a line is very likely a section header (date, or a status/time
// stamp we don't want mistaken for a transaction title) rather than an
// actual transaction line.
function isHeaderLike(line) {
  return (
    /^today\b/i.test(line) ||
    /^yesterday\b/i.test(line) ||
    EXPLICIT_DATE.test(line) ||
    EXPLICIT_DATE_MONTH_FIRST.test(line) ||
    (WEEKDAY_HEADER.test(line) && line.trim().length < 16)
  );
}

// Scans every line (not just ones before a transaction) for the last
// resolvable date. On a transaction-list screenshot this is redundant with
// the sequential header-tracking below. On a single-transaction receipt/
// detail screen — like a "Transaction Date: Jul 13th, 2026" field — the
// date usually appears *after* the amount, so the sequential scan alone
// would miss it; this catches it as a fallback.
function findDocumentDate(lines, now) {
  let found = null;
  for (const line of lines) {
    const d = parseDateHeader(line, now);
    if (d) found = d;
  }
  return found;
}

// The ₦ glyph is poorly represented in Tesseract's default "eng" trained
// data, so on real screenshots it very often gets OCR'd as a stray digit
// glued onto the number (e.g. "₦500.00" -> "4500.00") or dropped entirely,
// rather than misread as a letter. When that happens AMOUNT_RE finds
// nothing at all, since it requires a currency marker or a dr/cr suffix.
// A line whose *entire* trimmed content is just a currency-shaped number
// (grouped thousands, exactly 2 decimal places) is almost always the big
// standalone amount on a receipt/list row — never a phone number, time, or
// transaction ID, which don't take that exact shape — so treat it as an
// amount even with no symbol. The leading digit may occasionally be a
// misread-symbol artifact rather than a real digit; that's an acceptable
// trade next to returning zero transactions, and it's exactly what the
// review step before saving is for.
const BARE_AMOUNT_LINE = /^[\d][\d,]*\.\d{2}$/;

function extractAmount(line) {
  const trimmed = line.trim();
  const m = trimmed.match(AMOUNT_RE);

  let sign = null;
  let amountStr = null;

  if (m) {
    if (m[2] !== undefined) {
      sign = m[1] || null;
      amountStr = m[2];
    } else if (m[3] !== undefined) {
      amountStr = m[3];
      if (m[4]) sign = m[4].toLowerCase() === "dr" ? "-" : "+";
    } else if (m[5] !== undefined) {
      amountStr = m[5];
      if (m[6]) sign = m[6].toLowerCase() === "dr" ? "-" : "+";
    }
  } else if (BARE_AMOUNT_LINE.test(trimmed)) {
    amountStr = trimmed;
  }

  if (!amountStr) return null;
  const amount = parseFloat(amountStr.replace(/,/g, ""));
  if (!Number.isFinite(amount) || amount <= 0) return null;

  return { amount, sign, matchText: m ? m[0] : trimmed };
}

// Rough keyword -> category-name matching against whatever categories the
// user actually has (so renamed/custom categories still get matched). Falls
// back to "Other" if it exists, otherwise leaves it unset for the user to
// pick in review.
const KEYWORD_HINTS = [
  { keywords: ["airtime", "data bundle", "recharge", " mtn", " glo", "airtel", "9mobile"], category: "Data & Airtime" },
  { keywords: ["uber", "bolt", "taxi", "transport", "fuel", "fare"], category: "Transport" },
  { keywords: ["restaurant", "food", "kitchen", "eatery", "chow", "jumia food", "bolt food"], category: "Food" },
  { keywords: ["netflix", "spotify", "showmax", "cinema", "movie", "showtime"], category: "Entertainment" },
  { keywords: ["electricity", "phcn", "disco", "water bill", "utility", "utilities"], category: "Utilities" },
  { keywords: ["rent", "landlord"], category: "Rent" },
  { keywords: ["pharmacy", "hospital", "clinic", "drug", "health"], category: "Health" },
  { keywords: ["shoprite", "market", "store", "mall", "shopping"], category: "Shopping" },
  { keywords: ["salary", "payroll"], category: "Salary" },
  { keywords: ["transfer from", "received from", "deposit"], category: "Other" },
];

export function guessCategory(description, categories) {
  const lower = description.toLowerCase();
  const names = categories.map((c) => c.name);

  for (const hint of KEYWORD_HINTS) {
    if (hint.keywords.some((k) => lower.includes(k)) && names.includes(hint.category)) {
      return hint.category;
    }
  }
  return names.includes("Other") ? "Other" : "";
}

function cleanDescription(line, amountMatchText) {
  let text = line;
  if (amountMatchText) text = text.replace(amountMatchText, " ");
  return text
    .replace(/\b(successful|completed|pending|failed)\b/gi, " ")
    .replace(/\b\d{1,2}:\d{2}\s?(am|pm)?\b/gi, " ")
    .replace(/[|•·]+/g, " ")
    .replace(/\s{2,}/g, " ")
    .trim();
}

// Shared line-walker used by both bank parsers below. `now` is injectable
// for testing; defaults to the real current date.
function parseTransactionLines(rawText, categories, now = new Date()) {
  const lines = rawText
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);

  const drafts = [];
  let currentDate = now.toISOString().slice(0, 10);
  const defaultDate = currentDate;
  let pendingTitle = "";

  for (const line of lines) {
    if (NOISE_LINE.test(line)) {
      // Reward/cashback line — not a transaction of its own, and shouldn't
      // be picked up as a pending title for whatever comes next either.
      continue;
    }

    const headerDate = parseDateHeader(line, now);
    if (headerDate) {
      currentDate = headerDate;
      pendingTitle = "";
      continue;
    }
    if (isHeaderLike(line)) {
      // Recognized as a header we can't resolve to a date — skip rather
      // than mistaking it for a transaction title.
      continue;
    }

    const amountInfo = extractAmount(line);
    if (!amountInfo) {
      // No amount on this line — likely a title that's wrapped onto its
      // own line, with the amount appearing on the next one. Hold onto it.
      if (line.length >= 3) pendingTitle = line;
      continue;
    }

    let description = cleanDescription(line, amountInfo.matchText);
    if (!description && pendingTitle) description = pendingTitle;
    if (!description) description = "Transaction";
    pendingTitle = "";

    const type = amountInfo.sign === "+" ? "income" : "expense";

    drafts.push({
      id: `scan-${drafts.length}-${Math.random().toString(36).slice(2, 8)}`,
      include: true,
      date: currentDate,
      description,
      amount: amountInfo.amount,
      type,
      category: guessCategory(description, categories),
    });
  }

  // Single-transaction receipt/detail screens (as opposed to a scrollable
  // list) often show the date *after* the amount — e.g. a "Transaction
  // Date" field near the bottom. If there's exactly one result and its date
  // is still the unresolved default, prefer whatever date was found
  // anywhere else on screen over silently defaulting to "today".
  if (drafts.length === 1 && drafts[0].date === defaultDate) {
    const documentDate = findDocumentDate(lines, now);
    if (documentDate) drafts[0].date = documentDate;
  }

  // Single-transaction receipt/detail screens often spell out the kind of
  // transaction in a separate "Transaction Type" field (e.g. "Airtime",
  // "Transfer", "Bill Payment") that's more reliable for category-guessing
  // than the merchant name alone. Fold it in when present.
  if (drafts.length === 1) {
    const typeLine = lines.find((l) => /^transaction type\b/i.test(l));
    if (typeLine) {
      const value = typeLine.replace(/^transaction type\s*[:-]?\s*/i, "").trim();
      if (value) {
        const improved = guessCategory(`${drafts[0].description} ${value}`, categories);
        if (improved) drafts[0].category = improved;
      }
    }
  }

  return drafts;
}

export function parseOpayScreenshot(rawText, categories, now) {
  return parseTransactionLines(rawText, categories, now);
}

export function parseMoniepointScreenshot(rawText, categories, now) {
  return parseTransactionLines(rawText, categories, now);
}

export function parseBankScreenshot(bank, rawText, categories, now) {
  if (bank === "moniepoint") return parseMoniepointScreenshot(rawText, categories, now);
  return parseOpayScreenshot(rawText, categories, now);
}
