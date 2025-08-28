const path = window.location.pathname;
const domain = window.location.hostname;
const pathSegments = path.split('/').filter(segment => segment !== '');

const roomId = pathSegments[0] || 'default-room';
const streamerId = pathSegments[1] || 'default-streamer';

const urlParams = new URLSearchParams(window.location.search);

const mode = urlParams.get('mode') || 'publisher';
const jwt = urlParams.get('jwt') || '';

if (mode === 'viewer') {
    document.getElementById('localVideo').style.display = 'none';
}

let localStream = null;

const toggleCamera = () => {
    if (!localStream) return;
    localStream.getVideoTracks().forEach(track => {
        track.enabled = !track.enabled;
    });
};

const toggleMicrophone = () => {
    if (!localStream) return;
    localStream.getAudioTracks().forEach(track => {
        track.enabled = !track.enabled;
    });
};

const mediaPromise = mode === 'publisher'
    ? navigator.mediaDevices.getUserMedia({video: true, audio: true})
    : Promise.resolve(null);

function onClickUnmute(el, dom) {
    el.muted = false;
    console.log("Unmuting media element:", el);
    el.play().catch(error => {

        console.error("Error playing media:", error);
    });
    dom.removeEventListener('click', () => onClickUnmute(el, dom));
}


mediaPromise.then(stream => {
    let pc = new RTCPeerConnection({
        iceServers: [
            { urls: 'stun:stun.l.google.com:19302' },
            { urls: 'stun:stun1.l.google.com:19302' }
        ]
    });
    const trackElementMap = new Map();

    pc.ontrack = function (event) {
        const track = event.track;
        const stream = event.streams[0];

        let el = document.createElement(track.kind);
        el.srcObject = stream;
        el.autoplay = true;
        el.controls = false;
        el.muted = true;

        el.setAttribute('data-track-id', track.id);

        trackElementMap.set(track.id, el);

        const dom = document.querySelector(`body`);

        if (track.kind === 'audio') {
            console.log("Adding audio track:", track);
            window.addEventListener('click', () => onClickUnmute(el, dom));
        }

        dom.appendChild(el);
        track.addEventListener('ended', () => {
            console.log("Track ended:", track.kind, track.id);
            const elementToRemove = trackElementMap.get(track.id);
            if (elementToRemove) {
                elementToRemove.remove();
                trackElementMap.delete(track.id);
            }
        });
    };

    pc.ontrack = function (event) {
        const track = event.track;
        const stream = event.streams[0];

        console.log("Adding track:", track.kind, track.id);

        let el = document.createElement(track.kind);
        el.srcObject = stream;
        el.autoplay = true;
        el.controls = false;
        el.muted = true;

        const dom = document.querySelector(`body`);

        if (track.kind === 'audio') {
            console.log("Adding audio track:", track);
            window.addEventListener('click', () => onClickUnmute(el, dom));
        }

        dom.appendChild(el);

        const removeTrackHandler = (removeEvent) => {
            if (removeEvent.track.id === track.id) {
                console.log("Removing track:", removeEvent.track.kind, removeEvent.track.id);
                el.remove();
                stream.removeEventListener('removetrack', removeTrackHandler);
            }
        };

        stream.addEventListener('removetrack', removeTrackHandler);
    };

    if (mode === 'publisher' && stream) {
        localStream = stream;
        document.getElementById('localVideo').srcObject = stream;
        stream.getTracks().forEach(track => pc.addTrack(track, stream));
    }

    let ws;
    if(mode === 'viewer') {
        ws = new WebSocket(`ws://${domain}:9090/room/websocket/watch/${roomId}/${streamerId}?mode=${mode}`);
    }else {
        ws = new WebSocket(`wss://${domain}:9090/room/websocket/stream/${roomId}?mode=${mode}&jwt=${jwt}`);
    }


    pc.onicecandidate = e => {
        if (!e.candidate) return;
        ws.send(JSON.stringify({event: 'candidate', data: JSON.stringify(e.candidate)}));
    };

    ws.onclose = function () {
        console.log('Websocket has closed');
    };

    ws.onmessage = function (evt) {
        let msg = JSON.parse(evt.data);
        if (!msg) return;
        switch (msg.event) {
            case 'offer':
                let offer = JSON.parse(msg.data);
                if (!offer) return;
                pc.setRemoteDescription(offer);
                pc.createAnswer().then(answer => {
                    pc.setLocalDescription(answer);
                    ws.send(JSON.stringify({event: 'answer', data: JSON.stringify(answer)}));
                });
                break;
            case 'candidate':
                let candidate = JSON.parse(msg.data);
                if (!candidate) return;
                pc.addIceCandidate(candidate);
                break;
        }
    };
    ws.onerror = function (evt) {
        console.log('ERROR: ' + evt.data);
    };
});