export function uuidv7(): string {
  const ts = Date.now();
  const tsHex = ts.toString(16).padStart(12, '0');

  const rand = new Uint8Array(10);
  crypto.getRandomValues(rand);

  rand[0] = (rand[0] & 0x0f) | 0x70;
  rand[2] = (rand[2] & 0x3f) | 0x80;

  const randHex = Array.from(rand, (b) => b.toString(16).padStart(2, '0')).join('');

  return `${tsHex.slice(0, 8)}-${tsHex.slice(8, 12)}-${randHex.slice(0, 4)}-${randHex.slice(4, 8)}-${randHex.slice(8, 20)}`;
}