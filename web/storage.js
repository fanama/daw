// Local persistence for the browser build: project JSON and take audio live
// in IndexedDB, which is the browser counterpart of the session folder.

const DATABASE_NAME = "gorecord-browser";
const DATABASE_VERSION = 1;

let databasePromise = null;

function openDatabase() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DATABASE_NAME, DATABASE_VERSION);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains("sessions")) {
        database.createObjectStore("sessions", { keyPath: "root" });
      }
      if (!database.objectStoreNames.contains("takes")) {
        database.createObjectStore("takes", { keyPath: "key" });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

async function database() {
  if (!databasePromise) {
    databasePromise = openDatabase().catch((error) => {
      databasePromise = null;
      throw error;
    });
  }
  return databasePromise;
}

function requestResult(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

function transactionDone(transaction) {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve();
    transaction.onerror = () => reject(transaction.error);
    transaction.onabort = () => reject(transaction.error);
  });
}

async function put(storeName, value) {
  const transaction = (await database()).transaction(storeName, "readwrite");
  transaction.objectStore(storeName).put(value);
  await transactionDone(transaction);
  return value;
}

async function getAll(storeName) {
  const transaction = (await database()).transaction(storeName, "readonly");
  return requestResult(transaction.objectStore(storeName).getAll());
}

export async function listSessions() {
  const rows = await getAll("sessions");
  return rows.sort((left, right) => right.updatedAt - left.updatedAt);
}

export async function loadProject(root) {
  const transaction = (await database()).transaction("sessions", "readonly");
  return requestResult(transaction.objectStore("sessions").get(root));
}

export async function saveProject(root, project, name) {
  return put("sessions", { root, name, project, updatedAt: Date.now() });
}

export async function saveTake(root, file, channels, sampleRate, samples) {
  const copy = samples.buffer.slice(samples.byteOffset, samples.byteOffset + samples.byteLength);
  return put("takes", { key: `${root}|${file}`, root, file, channels, sampleRate, data: copy });
}

export async function loadTakes(root) {
  const rows = await getAll("takes");
  return rows
    .filter((row) => row.root === root)
    .map((row) => ({
      file: row.file,
      channels: row.channels,
      sampleRate: row.sampleRate,
      samples: new Float32Array(row.data),
    }));
}

export async function deleteTake(root, file) {
  const transaction = (await database()).transaction("takes", "readwrite");
  transaction.objectStore("takes").delete(`${root}|${file}`);
  await transactionDone(transaction);
}

export async function deleteSession(root) {
  const rows = await getAll("takes");
  const transaction = (await database()).transaction(["sessions", "takes"], "readwrite");
  transaction.objectStore("sessions").delete(root);
  for (const row of rows) {
    if (row.root === root) {
      transaction.objectStore("takes").delete(row.key);
    }
  }
  await transactionDone(transaction);
}
