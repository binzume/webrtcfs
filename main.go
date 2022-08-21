package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/binzume/webrtcfs/rtcfs"
	"github.com/binzume/webrtcfs/socfs"
)

type Config struct {
	SignalingUrl        string
	SignalingKey        string
	RoomIdPrefix        string
	PairingRoomIdPrefix string
	PairingTimeoutSec   int

	RoomName  string
	AuthToken string
	LocalPath string

	Writable bool

	ThumbnailCacheDir string
	FFmpegPath        string
}

func DefaultConfig() *Config {
	var config Config
	config.SignalingUrl = "wss://ayame-labo.shiguredo.app/signaling"
	config.SignalingKey = "VV69g7Ngx-vNwNknLhxJPHs9FpRWWNWeUzJ9FUyylkD_yc_F"
	config.RoomIdPrefix = "binzume@rdp-room-"
	config.PairingRoomIdPrefix = "binzume@rdp-pin-"
	config.PairingTimeoutSec = 600
	config.ThumbnailCacheDir = "cache"
	return &config
}

func loadConfig(confPath string) *Config {
	config := DefaultConfig()

	_, err := toml.DecodeFile(confPath, config)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("WARN: %s not found. use default settings.\n", confPath)
	} else if err != nil {
		log.Fatal("Failed to load ", confPath, err)
	}
	return config
}

func publishFiles(ctx context.Context, config *Config, options *rtcfs.ConnectOptions) error {
	if config.ThumbnailCacheDir != "" {
		socfs.DefaultThumbnailer.Register(socfs.NewImageThumbnailer(config.ThumbnailCacheDir))
		if config.FFmpegPath != "" {
			socfs.DefaultThumbnailer.Register(socfs.NewVideoThumbnailer(config.ThumbnailCacheDir, config.FFmpegPath))
		}
	}

	fsys := socfs.NewWritableDirFS(config.LocalPath)
	if !config.Writable {
		fsys.Capability().Create = false
		fsys.Capability().Remove = false
		fsys.Capability().Write = false
	}
	log.Println("connecting... ", options.RoomID)
	return rtcfs.Publish(ctx, options, fsys)
}

func main() {
	confPath := flag.String("conf", "config.toml", "conf path")
	roomName := flag.String("room", "", "Ayame room name")
	authToken := flag.String("token", "", "auth token")
	localPath := flag.String("path", ".", "local path to share")
	writable := flag.Bool("writable", false, "writable fs")
	flag.Parse()

	config := loadConfig(*confPath)
	if *localPath != "" {
		config.LocalPath = *localPath
	}
	if *roomName != "" {
		config.RoomName = *roomName
	}
	if *authToken != "" {
		config.AuthToken = *authToken
	}
	if *writable {
		config.Writable = *writable
	}

	options := &rtcfs.ConnectOptions{
		SignalingURL: config.SignalingUrl,
		SignalingKey: config.SignalingKey,
		RoomID:       config.RoomIdPrefix + config.RoomName + ".1",
		AuthToken:    config.AuthToken,
	}

	switch flag.Arg(0) {
	case "pairing":
		err := rtcfs.Pairing(context.Background(), &rtcfs.PairingOptions{
			ConnectOptions:      *options,
			PairingRoomIDPrefix: config.PairingRoomIdPrefix,
			Timeout:             time.Duration(config.PairingTimeoutSec) * time.Second,
		})
		if err != nil {
			log.Println(err)
		}
	case "shell":
		err := rtcfs.StartShell(context.Background(), options)
		if err != nil {
			log.Println(err)
		}
	case "pull", "push", "ls", "cat", "rm":
		err := rtcfs.ShellExec(context.Background(), options, flag.Arg(0), flag.Arg(1))
		if err != nil {
			log.Println(err)
		}
	case "publish", "":
		for {
			err := publishFiles(context.Background(), config, options)
			if err != nil {
				log.Println("ERROR:", err)
			}
			time.Sleep(5 * time.Second)
		}
	default:
		fmt.Println("Unknown sub command: ", flag.Arg(0))
		flag.Usage()
	}
}
