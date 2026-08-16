export function opfsAvailable(): boolean {
  return typeof navigator !== 'undefined'
    && typeof navigator.storage !== 'undefined'
    && typeof navigator.storage.getDirectory === 'function';
}

export async function readOpfsFile(name: string): Promise<Uint8Array | null> {
  const root = await navigator.storage.getDirectory();
  let handle: FileSystemFileHandle;
  try {
    handle = await root.getFileHandle(name);
  } catch (err) {
    if (err instanceof DOMException && err.name === 'NotFoundError') return null;
    throw err;
  }
  const file = await handle.getFile();
  const buf = await file.arrayBuffer();
  return new Uint8Array(buf);
}

export async function writeOpfsFile(name: string, data: Uint8Array): Promise<void> {
  const root = await navigator.storage.getDirectory();
  const handle = await root.getFileHandle(name, { create: true });
  const writable = await handle.createWritable();
  const buf = data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength);
  await writable.write(buf as ArrayBuffer);
  await writable.close();
}

export async function renameOpfsFile(from: string, to: string): Promise<void> {
  const root = await navigator.storage.getDirectory();
  let fromHandle: FileSystemFileHandle;
  try {
    fromHandle = await root.getFileHandle(from);
  } catch (err) {
    if (err instanceof DOMException && err.name === 'NotFoundError') return;
    throw err;
  }
  const file = await fromHandle.getFile();
  const data = new Uint8Array(await file.arrayBuffer());
  const toHandle = await root.getFileHandle(to, { create: true });
  const writable = await toHandle.createWritable();
  const buf = data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength);
  await writable.write(buf as ArrayBuffer);
  await writable.close();
  await root.removeEntry(from);
}

export async function removeOpfsFile(name: string): Promise<void> {
  const root = await navigator.storage.getDirectory();
  try {
    await root.removeEntry(name);
  } catch (err) {
    if (err instanceof DOMException && err.name === 'NotFoundError') return;
    throw err;
  }
}

export function sidecarName(prefix: string): string {
  const ts = new Date().toISOString().replace(/[:.]/g, '-');
  return `${prefix}${ts}.db`;
}