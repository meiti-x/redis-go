Here’s an updated version of your README with the new **Stream** commands (`XADD`, `XRANGE`, `XREAD`) added, staying consistent with your tone and structure:

---

# 🚀 RedisGO - A Mini Redis Clone in Go

**Just for fun!** A toy Redis server implementation supporting basic commands with RESP protocol.

---

## 🌟 Features (until now 😄)

| Command     | Example                       | Description             |
| ----------- | ----------------------------- | ----------------------- |
| `PING`      | `PING` → `PONG`               | Health check            |
| `ECHO`      | `ECHO "Hi"` → `"Hi"`          | Echoes messages         |
| `SET`       | `SET name "Alice"`            | Stores key-value        |
|             | `SET age 25 PX 5000`          | With TTL (ms)           |
| `GET`       | `GET name` → `"Alice"`        | Retrieves values        |
| `TYPE`      | `TYPE name` → `string`        | Checks key type         |
| *(Passive)* | *Auto-expires keys on access* |                         |
| *(Active)*  | *Background expiry scanner*   |                         |
| `XADD`      | `XADD mystream * name Alice`  | Appends entry to stream |
| `XRANGE`    | `XRANGE mystream - +`         | Reads range of entries  |
| `XREAD`     | `XREAD STREAMS mystream 0`    | Reads new entries by ID |

---

## 🛠️ Tech Stack

* **100% Go** (no dependencies)
* **RESP Protocol** (Redis Serialization)
* **Concurrent Safe** (RWMutex)
* **Dual Expiry**:

  * Active: Background cleaner
  * Passive: On-access checks
* **Stream Support**: Simple `XADD`, `XRANGE`, `XREAD` with basic ID handling

---

## 🎯 Why This Exists

* Learn Redis internals
* Experiment with Go concurrency
* Because building things > using things

---

🔥 **Warning**: Not production-ready! Missing 99% of Redis features 😅.

---

Let me know if you want to add usage examples for stream commands or diagrams for internal structure!
