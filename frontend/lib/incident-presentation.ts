export function formatIncidentTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";

  return `${new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
    timeZone: "UTC",
  }).format(date)} UTC`;
}

export function formatDowntime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";

  let remaining = Math.floor(seconds);
  const parts: string[] = [];
  for (const [unit, size] of [["d", 86400], ["h", 3600], ["m", 60]] as const) {
    const amount = Math.floor(remaining / size);
    if (amount > 0) parts.push(`${amount}${unit}`);
    remaining %= size;
  }
  if (remaining > 0 || parts.length === 0) parts.push(`${remaining}s`);
  return parts.join(" ");
}
