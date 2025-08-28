package websocket

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"livestream/app/config"
	"livestream/app/firebase/livedatabase"
	"time"

	"github.com/gofiber/fiber/v2/log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

var (
	keyFrameDispatcher *time.Ticker
	keyFrameContext    context.Context
	keyFrameCancel     context.CancelFunc

	roomCleanupTicker  *time.Ticker
	roomCleanupContext context.Context
	roomCleanupCancel  context.CancelFunc
)

func livestreamSocketStreamer(c *websocket.Conn) {
	log.Info("Streamer connected")
	roomID := c.Params("roomID")
	userId := c.Locals("id").(string)
	room, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		_ = c.Close()
		return
	}
	log.Infof("Streamer connected to room %s", roomID)
	HandleWebRTCConnectionRoomStreamer(c, room, userId)
}

func livestreamSocketViewer(c *websocket.Conn) {
	roomID := c.Params("roomID")
	streamerID := c.Params("streamerID")

	log.Infof("Viewer connected to room %s", roomID)

	room, exists := GetRoom(roomID)
	if !exists {
		_ = c.Close()
		return
	}
	HandleWebRTCConnectionRoomSViewer(c, room, streamerID)
}

func toggleCamera(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	userId := c.Locals("id").(string)

	room, exists := GetRoom(roomID)
	if exists == false {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}

	if room.Paused {
		return c.JSON(fiber.Map{
			"success":        true,
			"camera_enabled": false,
		})
	}

	streamer := room.GetStreamer(userId)
	if err := streamer.ToggleCamera(); err != nil {
		log.Errorf("Failed to toggle camera for streamer %s in room %s: %v", userId, roomID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to toggle camera",
		})
	}

	return c.JSON(fiber.Map{
		"success":        true,
		"camera_enabled": streamer.CameraEnabled,
	})
}

func toggleMicrophone(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	streamerID := c.Locals("id").(string)

	room, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}
	if room.Paused {
		return c.JSON(fiber.Map{
			"success":            true,
			"microphone_enabled": false,
		})
	}
	streamer := room.GetStreamer(streamerID)

	if err := streamer.ToggleMicrophone(); err != nil {
		log.Errorf("Failed to toggle microphone for streamer %s in room %s: %v", streamerID, roomID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to toggle microphone",
		})
	}

	return c.JSON(fiber.Map{
		"success":            true,
		"microphone_enabled": streamer.MicrophoneEnabled,
	})
}

func pauseAllStreamers(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	currentUserId := c.Locals("id").(string)

	room, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}

	if room.CreatedBy != currentUserId {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Only room creator can pause all streamers",
		})
	}
	pauseStream(room)
	return c.JSON(fiber.Map{
		"success": true,
		"message": "All streamers paused successfully",
	})
}

func resumeAllStreamers(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	currentUserId := c.Locals("id").(string)

	room, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}

	if room.CreatedBy != currentUserId {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Only room creator can resume all streamers",
		})
	}

	resumeStream(room)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "All streamers resumed successfully",
	})
}

func createRoom(c *fiber.Ctx) error {
	log.Info("Creating room")
	randomId := uuid.New().String()
	userId := c.Locals("id").(string)
	jwt := c.Locals("jwt").(string)

	CreateRoom(randomId, userId)

	log.Infof("Room created: %s", randomId)

	return c.JSON(fiber.Map{
		"startStreamLink": fmt.Sprintf("%s/%s?jwt=%s&mode=publisher", config.BaseLink, randomId, jwt),
		"roomID":          randomId,
	})
}

func addStreamer(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	userId := c.Params("userId")

	currentUserId := c.Locals("id").(string)

	room, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}

	if room.CreatedBy != currentUserId {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	room.GetOrCreateStreamer(userId)

	if err := livedatabase.SendLivestreamInvitation(userId, roomID, currentUserId); err != nil {
		log.Errorf("Failed to send livestream invitation: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to send invitation",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Streamer added and invitation sent",
	})
}

func getStreamerLink(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	currentUserId := c.Locals("id").(string)
	jwt := c.Locals("jwt").(string)
	room, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}
	streamer := room.GetStreamer(currentUserId)
	if streamer == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User is not a streamer in this room",
		})
	}
	return c.JSON(fiber.Map{
		"startStreamLink": fmt.Sprintf("%s/%s?jwt=%s&mode=publisher", config.BaseLink, roomID, jwt),
	})
}

func joinRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	userID := c.Locals("id").(string)

	_, exists := GetRoom(roomID)
	if !exists {
		log.Errorf("Room %s not found", roomID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Room not found",
		})
	}

	user, err := GetUserById(userID)
	if err != nil {
		log.Errorf("Failed to get user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get user information",
		})
	}

	if err := livedatabase.AddWatcher(roomID, userID, user.Username, user.Avatar); err != nil {
		log.Errorf("Failed to add watcher %s to room %s: %v", userID, roomID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to join room",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Successfully joined room",
	})
}

func leaveRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomID")
	userID := c.Locals("id").(string)

	if err := livedatabase.RemoveWatcher(roomID, userID); err != nil {
		log.Errorf("Failed to remove watcher %s from room %s: %v", userID, roomID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to leave room",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Successfully left room",
	})
}

func Register(router fiber.Router) {
	router.Post("/join/:roomID", joinRoom)
	router.Post("/leave/:roomID", leaveRoom)

	router.Post("/", createRoom)
	router.Post("/add-streamer/:roomID/:userId", addStreamer)
	router.Get("/streamer-link/:roomID", getStreamerLink)

	router.Post("/:roomID/camera/toggle", toggleCamera)
	router.Post("/:roomID/microphone/toggle", toggleMicrophone)

	router.Post("/:roomID/pause", pauseAllStreamers)
	router.Post("/:roomID/resume", resumeAllStreamers)

	group := router.Group("/websocket")
	group.Get("/stream/:roomID", websocket.New(livestreamSocketStreamer))
	group.Get("/watch/:roomID/:streamerID", websocket.New(livestreamSocketViewer))

	startKeyFrameDispatcher()
	//startRoomCleanupTask()
}

func startKeyFrameDispatcher() {
	keyFrameContext, keyFrameCancel = context.WithCancel(context.Background())
	keyFrameDispatcher = time.NewTicker(3 * time.Second)

	go func() {
		defer keyFrameDispatcher.Stop()

		for {
			select {
			case <-keyFrameContext.Done():
				return
			case <-keyFrameDispatcher.C:
				dispatchKeyFrameAllRooms()
			}
		}
	}()
}

func StopKeyFrameDispatcher() {
	if keyFrameCancel != nil {
		keyFrameCancel()
	}
}

func startRoomCleanupTask() {
	roomCleanupContext, roomCleanupCancel = context.WithCancel(context.Background())
	roomCleanupTicker = time.NewTicker(5 * time.Minute) // Check every 5 minutes

	go func() {
		defer roomCleanupTicker.Stop()

		for {
			select {
			case <-roomCleanupContext.Done():
				return
			case <-roomCleanupTicker.C:
				CleanupInactiveRooms(10 * time.Minute)
			}
		}
	}()
}

func StopRoomCleanupTask() {
	if roomCleanupCancel != nil {
		roomCleanupCancel()
	}
}
