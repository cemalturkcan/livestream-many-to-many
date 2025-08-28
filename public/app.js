function getVideoCount() {
    const videos = document.querySelectorAll('video');
    return videos.length;
}

navigator.mediaDevices.getUserMedia({ video: true, audio: true })
    .then(stream => {
        let pc = new RTCPeerConnection();
        pc.ontrack = function (event) {
            if (event.track.kind === 'audio') return;
            let el = document.createElement(event.track.kind);
            el.srcObject = event.streams[0];
            el.autoplay = true;
            el.controls = false;
            el.muted = true;

            let witchContainer;

            if (getVideoCount() === 1){
                document.getElementById('videos-one').appendChild(el);
                witchContainer = 'videos-one';
            }else {
                const videosTwo = document.getElementById('videos-two');
                videosTwo.classList.add('flex-1');
                videosTwo.appendChild(el);
                witchContainer = 'videos-two';
            }

            event.track.onmute = function() { el.play(); };
            event.streams[0].onremovetrack = ({track}) => {
                if (el.parentNode) {
                    if (witchContainer === 'videos-two') {
                        const videosTwo = document.getElementById('videos-two');
                        if (getVideoCount() === 2) {
                            videosTwo.classList.remove('flex-1');
                        }
                    }
                    el.parentNode.removeChild(el);
                }
            };
        };
        document.getElementById('localVideo').srcObject = stream;
        stream.getTracks().forEach(track => pc.addTrack(track, stream));
        const path = window.location.pathname;
        const roomId = path.split('/').pop() || 'default-room';
        let ws = new WebSocket(`ws://127.0.0.1:9090/api/websocket/${roomId}`);
        pc.onicecandidate = e => {
            if (!e.candidate) return;
            ws.send(JSON.stringify({event: 'candidate', data: JSON.stringify(e.candidate)}));
        };

        ws.onclose = function() { console.log('Websocket has closed'); };

        ws.onmessage = function(evt) {
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
        ws.onerror = function(evt) { console.log('ERROR: ' + evt.data); };
    })