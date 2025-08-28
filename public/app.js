class LivestreamApp {
    constructor() {
        this.pc = null;
        this.ws = null;
        this.localStream = null;
        this.isViewer = false;
        this.currentRoomId = null;
        this.videos = new Map();
        
        this.initializeUI();
        this.setupEventListeners();
    }

    initializeUI() {
        this.videosContainer = document.getElementById('videos-container');
        this.sidebar = document.getElementById('sidebar');
        this.toggleButton = document.getElementById('toggle-sidebar');
        this.roomIdInput = document.getElementById('room-id');
        this.broadcasterBtn = document.getElementById('broadcaster-btn');
        this.viewerBtn = document.getElementById('viewer-btn');
        this.joinButton = document.getElementById('join-room');
        this.statusIndicator = document.getElementById('status-indicator');
        this.statusDot = document.getElementById('status-dot');
        this.statusText = document.getElementById('status-text');

        const path = window.location.pathname;
        const urlRoomId = path.split('/').pop();
        if (urlRoomId && urlRoomId !== '') {
            this.roomIdInput.value = urlRoomId;
        }
    }

    setupEventListeners() {
        this.broadcasterBtn.addEventListener('click', () => {
            this.setRole(false);
        });

        this.viewerBtn.addEventListener('click', () => {
            this.setRole(true);
        });

        this.joinButton.addEventListener('click', () => {
            this.joinRoom();
        });
    }

    setRole(isViewer) {
        this.isViewer = isViewer;
        this.broadcasterBtn.classList.toggle('active', !isViewer);
        this.viewerBtn.classList.toggle('active', isViewer);
    }

    async joinRoom() {
        const roomId = this.roomIdInput.value.trim();
        if (!roomId) {
            this.updateStatus('Room ID required', false);
            return;
        }

        this.currentRoomId = roomId;
        this.joinButton.disabled = true;
        this.updateStatus('Connecting...', false);

        try {
            if (!this.isViewer) {
                await this.initializeBroadcaster();
            } else {
                this.initializeViewer();
            }
            this.connectWebSocket();
        } catch (error) {
            this.updateStatus('Connection error', false);
            this.joinButton.disabled = false;
        }
    }

    async initializeBroadcaster() {
        this.localStream = await navigator.mediaDevices.getUserMedia({ 
            video: true, 
            audio: true 
        });
        this.addVideoElement(this.localStream, 'You', true);
        this.setupPeerConnection();
        this.localStream.getTracks().forEach(track => 
            this.pc.addTrack(track, this.localStream)
        );
    }

    initializeViewer() {
        this.setupPeerConnection();
    }

    setupPeerConnection() {
        this.pc = new RTCPeerConnection({
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' }
            ]
        });

        this.pc.ontrack = (event) => {
            if (event.track.kind === 'video') {
                this.addVideoElement(event.streams[0], `User ${this.videos.size + 1}`);
            }
        };

        this.pc.onicecandidate = (event) => {
            if (event.candidate && this.ws) {
                this.ws.send(JSON.stringify({
                    event: 'candidate',
                    data: JSON.stringify(event.candidate)
                }));
            }
        };

        this.pc.onconnectionstatechange = () => {
            const state = this.pc.connectionState;
            if (state === 'connected') {
                this.updateStatus('Connected', true);
            } else if (state === 'disconnected' || state === 'failed') {
                this.updateStatus('Disconnected', false);
            }
        };
    }

    connectWebSocket() {
        this.ws = new WebSocket(`ws://127.0.0.1:9090/api/websocket/${this.currentRoomId}`);

        this.ws.onopen = () => {
            this.updateStatus('WebSocket connected', true);
            this.sidebar.classList.add('hidden');
        };

        this.ws.onmessage = async (event) => {
            const msg = JSON.parse(event.data);
            await this.handleWebSocketMessage(msg);
        };

        this.ws.onclose = () => {
            this.updateStatus('Bağlantı kesildi', false);
            this.joinButton.disabled = false;
        };

        this.ws.onerror = () => {
            this.updateStatus('Connection error', false);
            this.joinButton.disabled = false;
        };
    }

    async handleWebSocketMessage(msg) {
        if (!msg || !this.pc) return;

        switch (msg.event) {
            case 'offer':
                const offer = JSON.parse(msg.data);
                if (!offer) return;
                await this.pc.setRemoteDescription(offer);
                const answer = await this.pc.createAnswer();
                await this.pc.setLocalDescription(answer);
                this.ws.send(JSON.stringify({
                    event: 'answer',
                    data: JSON.stringify(answer)
                }));
                break;

            case 'candidate':
                const candidate = JSON.parse(msg.data);
                if (candidate) {
                    await this.pc.addIceCandidate(candidate);
                }
                break;
        }
    }

    addVideoElement(stream, label, isLocal = false) {
        const videoWrapper = document.createElement('div');
        videoWrapper.className = 'video-wrapper';
        
        const video = document.createElement('video');
        video.srcObject = stream;
        video.autoplay = true;
        video.playsInline = true;
        video.muted = isLocal;
        
        const overlay = document.createElement('div');
        overlay.className = 'video-overlay';
        overlay.textContent = label;
        
        videoWrapper.appendChild(video);
        videoWrapper.appendChild(overlay);
        this.videosContainer.appendChild(videoWrapper);
        
        this.videos.set(stream.id, videoWrapper);
        this.updateVideoLayout();

        stream.addEventListener('removetrack', () => {
            this.removeVideoElement(stream.id);
        });
    }

    removeVideoElement(streamId) {
        const videoWrapper = this.videos.get(streamId);
        if (videoWrapper && videoWrapper.parentNode) {
            videoWrapper.parentNode.removeChild(videoWrapper);
            this.videos.delete(streamId);
            this.updateVideoLayout();
        }
    }

    updateVideoLayout() {
        const count = this.videos.size;
        this.videosContainer.className = `videos-container count-${Math.min(count, 4)}`;
    }

    updateStatus(text, connected) {
        this.statusText.textContent = text;
        this.statusDot.classList.toggle('connected', connected);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new LivestreamApp();
});