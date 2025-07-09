package common

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func CreateDirIfNotExists(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			log.Error().Str("dir", dir).Err(err).Msg("Error creating directory")
		}
	}
}

func IsOlderThan(filePath string, minutes int) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		log.Error().Str("filePath", filePath).Err(err).Msg("Error checking file modification time")
	}
	duration := time.Since(info.ModTime())
	return duration > time.Duration(minutes)*time.Minute
}

func MoveFile(src, dst string) {
	err := os.Rename(src, dst)
	if err != nil {
		log.Error().Str("src", src).Str("dst", dst).Err(err).Msg("Error moving file")
	}
}

func ChangeOwnership(filePath, ownerGroup string) {
	parts := strings.Split(ownerGroup, ":")
	username := parts[0]
	var groupname string

	// Determine if group is provided
	if len(parts) > 1 {
		groupname = parts[1]
	}

	usr, err := user.Lookup(username)
	if err != nil {
		log.Error().Str("username", username).Err(err).Msg("Error looking up user")
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		log.Error().Str("uid", usr.Uid).Err(err).Msg("Error converting UID to integer")
	}

	var gid int
	if groupname != "" {
		// Lookup specified group
		group, err := user.LookupGroup(groupname)
		if err != nil {
			log.Error().Str("groupname", groupname).Err(err).Msg("Error looking up group")
		}
		gid, err = strconv.Atoi(group.Gid)
		if err != nil {
			log.Error().Str("gid", group.Gid).Err(err).Msg("Error converting GID to integer")
		}
	} else {
		// Use the user's primary group
		gid, err = strconv.Atoi(usr.Gid)
		if err != nil {
			log.Error().Str("gid", usr.Gid).Err(err).Msg("Error converting primary GID to integer")
		}
	}

	err = os.Chown(filePath, uid, gid)
	if err != nil {
		log.Error().Str("filePath", filePath).Err(err).Msg("Error changing ownership of file")
	}
}
