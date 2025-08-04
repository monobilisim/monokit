package common

import (
	"fmt"
	"io"
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

func moveFileManual(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("couldn't open source file: %s", err)
	}

	out, err := os.Create(dst)
	if err != nil {
		in.Close()
		return fmt.Errorf("couldn't open dest file: %s", err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	in.Close()
	if err != nil {
		return fmt.Errorf("writing to output file failed: %s", err)
	}

	err = out.Sync()
	if err != nil {
		return fmt.Errorf("sync error: %s", err)
	}

	si, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat error: %s", err)
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return fmt.Errorf("chmod error: %s", err)
	}

	err = os.Remove(src)
	if err != nil {
		return fmt.Errorf("failed removing original file: %s", err)
	}
	return nil
}

func MoveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err != nil {
		log.Debug().Str("src", src).Str("dst", dst).Err(err).Msg("Error renaming file, possibly cross-device link. Using moveFileManual instead")
		err = moveFileManual(src, dst)
		if err != nil {
			log.Error().Str("src", src).Str("dst", dst).Err(err).Msg("Error moving file")
			return err
		}
	}
	return nil
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
