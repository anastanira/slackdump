# Port and traffic monitoring app concept (macOS + iPhone)

This document outlines a realistic architecture for monitoring TCP connections and recording traffic across macOS and iPhone devices. iOS restrictions make full traffic capture on-device infeasible without a VPN-based approach.

## Goals

- Detect inbound TCP connections to any local port.
- Detect outbound TCP connections initiated by local processes.
- Alert when a connection is accepted or initiated by any process.
- Record traffic for later review.
- Cover both macOS and iPhone devices.
- Provide a dashboard with understandable summaries for non-technical users.

## Reality check: platform constraints

### macOS

macOS allows deep inspection with the right privileges. You can implement full network monitoring using:

- **Network Extension (NEFilterDataProvider)** for per-process traffic filtering and inspection.
- **Packet capture (libpcap)** or **`pf`** firewall logging for port-level events.
- **EndpointSecurity framework** for process-level events (can correlate process and socket activity).

This requires a signed app with the appropriate entitlements and, for some APIs, a system extension.

### iOS / iPhone

iOS does **not** allow arbitrary packet sniffing. The only viable option is a **VPN-based** approach using **Network Extension (NEPacketTunnelProvider)**. This allows you to route device traffic through a local VPN tunnel and inspect packets in user space.

Key limitations:

- You cannot see traffic that bypasses the VPN (e.g., if the VPN is off).
- You cannot read traffic that is end-to-end encrypted beyond basic metadata (SNI, destination IP/port, flow sizes).
- You cannot inspect traffic from other apps unless they are routed through the VPN.

## Proposed architecture

### 1) macOS agent (system extension + helper)

**Responsibilities**:

- Monitor inbound and outbound TCP connections (all ports).
- Map connection to owning process.
- Capture traffic payload when permitted.
- Send alerts to a local UI or a backend.

**Implementation options**:

- **Network Extension Filter** (preferred):
  - Use `NEFilterDataProvider` to inspect per-flow data.
  - Record data chunks and metadata (source IP, port, PID).
  - Allows per-app decision making and alerts.

- **Packet capture**:
  - Use `libpcap` or `pcap` with BPF filters.
  - Use `pf` to log inbound connections and parse logs.
  - Map connections to processes using `proc_pidfdinfo` and socket tables (`nettop`, `lsof`-style logic).

**Alerting**:

- Local notifications (UserNotifications framework).
- Optional backend push (APNs) if you also have a server.

### 2) iOS agent (VPN tunnel provider)

**Responsibilities**:

- Establish a packet tunnel and act as a user-space router.
- Inspect packet headers for inbound and outbound connections to local ports.
- Provide connection metadata to the UI and/or backend.

**Implementation**:

- `NEPacketTunnelProvider` reads packets from `packetFlow.readPackets`.
- Decode IP/TCP headers to detect new inbound flows and outbound initiations.
- Log metadata and optionally store a rolling buffer of packet payloads.

**Constraints**:

- Packet capture is limited to what goes through the VPN tunnel.
- Cannot access other app’s traffic unless routed through the VPN.

## Dashboard and explanations

A dashboard should summarize each flow with a non-technical explanation alongside raw metadata.

**Suggested columns**:

- Date
- Time
- Direction (Inbound/Outbound)
- Process / App
- Local port
- Remote IP / host
- Protocol
- Packet summary (counts, bytes)
- Plain-language explanation (e.g., "Safari requested encrypted web content from example.com")

**Explanation pipeline**:

- Map destination IPs to domains where possible.
- Tag common protocols (HTTPS, DNS, MQTT, etc.).
- Provide templated explanations based on protocol + app.

## Storage and recording strategy

- Use **rolling buffers** to avoid unbounded storage growth.
- Store:
  - Flow metadata (timestamp, direction, local port, remote IP, PID if available).
  - Sampled payload chunks (e.g., first N KB or specific intervals).
- For sensitive data, ensure data is encrypted at rest.

## Pattern analysis and AI assistance

AI can help summarize traffic and highlight patterns, but it must respect privacy and platform limits.

**Suggested AI features**:

- Cluster similar flows by destination, app, and protocol to identify normal baselines.
- Flag unusual destinations, uncommon ports, or sudden spikes in outbound volume.
- Generate non-technical summaries for the dashboard explanation column.

**Network-wide monitoring**:

To record traffic from other devices on the same network, you need infrastructure that can observe that traffic (e.g., a router or gateway you control). The macOS/iOS app alone cannot see all LAN traffic by default. If you control the router:

- Mirror traffic (SPAN) or use a gateway/VPN on the router.
- Ingest flows into a central analyzer for cross-device pattern detection.
- Present per-device summaries in the same dashboard.

## Security and privacy considerations

- Use strict user consent and clear UI to show what is being monitored.
- Avoid storing payloads by default; allow users to opt-in.
- Provide a secure export and deletion workflow.

## Minimal viable feature set

### macOS

1. Capture inbound and outbound TCP connection metadata.
2. Alert on any new connection.
3. Record short payload segments.
4. Show connection history in a dashboard with explanations.

### iOS

1. VPN tunnel to capture packets.
2. Alert on inbound and outbound connection metadata.
3. Store only metadata initially.

## Suggested next steps

- Build a macOS prototype using Network Extension to validate feasibility.
- Build an iOS VPN prototype to validate what traffic is visible.
- Decide on storage model and UI design.
- Expand to a unified backend only if necessary.
