export function normalizeUrl(input: string): string {
  const v = input.trim();
  if (v === '') return '';
  if (/^[a-z][a-z0-9+\-.]*:/i.test(v)) return v;
  if (v.startsWith('//')) return `https:${v}`;
  return `https://${v}`;
}