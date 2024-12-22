package common

import (
    "os"
    "time"
    "os/user"
    "strconv"
    "strings"
)

func CreateDirIfNotExists(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
            LogError("Error creating directory: " + err.Error())
		}
	}
}

func IsOlderThan(filePath string, minutes int) bool {
	info, err := os.Stat(filePath)
	if err != nil {
        LogError("Error checking file modification time: " + err.Error())
	}
	duration := time.Since(info.ModTime())
	return duration > time.Duration(minutes)*time.Minute
}


func MoveFile(src, dst string) {
	err := os.Rename(src, dst)
	if err != nil {
        LogError("Error moving file: " + err.Error())
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
        LogError("Error looking up user: " + err.Error())
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
        LogError("Error converting UID to integer: " + err.Error())
	}

	var gid int
	if groupname != "" {
		// Lookup specified group
		group, err := user.LookupGroup(groupname)
		if err != nil {
            LogError("Error looking up group: " + err.Error())
		}
		gid, err = strconv.Atoi(group.Gid)
		if err != nil {
            LogError("Error converting GID to integer: " + err.Error())
		}
	} else {
		// Use the user's primary group
		gid, err = strconv.Atoi(usr.Gid)
		if err != nil {
            LogError("Error converting primary GID to integer: " + err.Error())
		}
	}

	err = os.Chown(filePath, uid, gid)
	if err != nil {
        LogError("Error changing ownership of file: " + err.Error())
	}
}
