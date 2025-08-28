# 🎥 Livestream Many-to-Many

A professional real-time video streaming platform enabling multiple users to broadcast and view live video streams simultaneously. Built with enterprise-grade WebRTC technology for direct peer-to-peer communication.

![Main Interface](screenshots/main-interface.png)

## 🏗️ Technology Stack

**Backend:**
- **Go 1.24.3** - High-performance server runtime
- **Fiber v2** - Express-inspired web framework
- **Pion WebRTC** - Pure Go WebRTC implementation
- **WebSocket** - Real-time bidirectional communication
- **Goroutines** - Concurrent connection handling

**Frontend:**
- **Vanilla JavaScript** - No framework dependencies
- **WebRTC API** - Browser-native peer-to-peer streaming
- **CSS Grid/Flexbox** - Modern responsive layouts
- **HTML5 Video** - Native video element handling

## ✨ Core Features

### Real-Time Video Communication
Multi-participant video streaming with low-latency peer-to-peer connections.

![Video Grid Layout](screenshots/video-grid.png)

### Role-Based Access
Broadcaster and viewer modes for flexible participation control.

![Role Selection](screenshots/role-selection.png)

## 🚀 Quick Start

### Prerequisites
- Go 1.24.3+
- Modern browser with WebRTC support

### Installation
```bash
git clone https://github.com/yourusername/livestream-many-to-many.git
cd livestream-many-to-many
go mod tidy
make run
```

Access at `http://localhost:9090`

![Installation](screenshots/installation.png)

## 📋 Usage

**Room Creation:** Enter unique room identifier and select broadcaster/viewer role.

**Broadcasting:** Grant camera permissions to share live video stream.

**Viewing:** Join existing rooms to watch live content without broadcasting.

![Usage Flow](screenshots/usage-flow.png)

## 🏛️ Architecture

Enterprise-grade signaling server built with Go Fiber handling WebSocket connections for WebRTC peer coordination. Frontend utilizes native browser WebRTC APIs for direct media streaming.

![Architecture](screenshots/architecture.png)

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.