package workers

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"github.com/CiscoCloud/distributive/wrkutils"
	"os"
	"strings"
	"syscall"
)

type fileTypeCheck func(path string) (bool, error)

// isType checks if the resource at path is of the type specified by name by
// passing path to checker. Mostly used to abstract Directory, File, Symlink.
func isType(name string, checker fileTypeCheck, path string) (exitCode int, message string) {
	boo, err := checker(path)
	if os.IsNotExist(err) {
		return 1, "No such file or directory: " + path
	}
	if os.IsPermission(err) {
		return 1, "Insufficient permissions to read: " + path
	}
	if boo {
		return 0, ""
	}
	return 1, "Is not a " + name + ": " + path
}

func isSymlink(path string) (bool, error) {
	_, err := os.Readlink(path)
	if err == nil {
		return true, err
	}
	return false, err
}

// file checks to see if the given path represents a normal file
func file(parameters []string) (exitCode int, exitMessage string) {
	// returns true if there is a regular ol' file at path
	// TODO fails on /dev/null
	isFile := func(path string) (bool, error) {
		if is, _ := isSymlink(path); is {
			return false, nil
		}
		fileInfo, err := os.Stat(path)
		if fileInfo == nil || !fileInfo.Mode().IsRegular() {
			return false, err
		}
		return true, err
	}
	return isType("file", isFile, parameters[0])
}

// directory checks to see if a directory exists at the specified path
func directory(parameters []string) (exitCode int, exitMessage string) {
	isDirectory := func(path string) (bool, error) {
		if is, _ := isSymlink(path); is {
			return false, nil
		}
		fileInfo, err := os.Stat(path)
		if fileInfo == nil || !fileInfo.Mode().IsDir() {
			return false, err
		}
		return true, err
	}
	return isType("directory", isDirectory, parameters[0])
}

// symlink checks to see if a symlink exists at a given path
func symlink(parameters []string) (exitCode int, exitMessage string) {
	// isSymlink checks to see if a symlink exists at this path.
	return isType("symlink", isSymlink, parameters[0])
}

// checksum checks the hash of a given file using the given algorithm
func checksum(parameters []string) (exitCode int, exitMessage string) {
	// getChecksum returns the checksum of some data, using a specified
	// algorithm
	getChecksum := func(algorithm string, data []byte) (checksum string) {
		algorithm = strings.ToUpper(algorithm)
		// default
		hasher := md5.New()
		switch algorithm {
		case "SHA1":
			hasher = sha1.New()
		case "SHA224":
			hasher = sha256.New224()
		case "SHA256":
			hasher = sha256.New()
		case "SHA384":
			hasher = sha512.New384()
		case "SHA512":
			hasher = sha512.New()
		}
		hasher.Write(data)
		str := hex.EncodeToString(hasher.Sum(nil))
		return str

	}
	// getFileChecksum is self-explanatory
	getFileChecksum := func(algorithm string, path string) (checksum string) {
		return getChecksum(algorithm, wrkutils.FileToBytes(path))
	}

	algorithm := parameters[0]
	checkAgainst := parameters[1]
	path := parameters[2]
	chksum := getFileChecksum(algorithm, path)
	// TODO warn on unequal lengths
	if chksum == checkAgainst {
		return 0, ""
	}
	msg := "Checksums do not match for file: " + path
	return wrkutils.GenericError(msg, checkAgainst, []string{chksum})
}

// fileContains checks whether a file matches a given regex
func fileContains(parameters []string) (exitCode int, exitMessage string) {
	path := parameters[0]
	regex := wrkutils.ParseUserRegex(parameters[1])
	if regex.Match(wrkutils.FileToBytes(path)) {
		return 0, ""
	}
	msg := "File does not match regexp:\n\tFile: "
	return 1, msg + path + "\n\tRegexp" + regex.String()
}

// permissions checks to see if a file's octal permissions match the given set
func permissions(parameters []string) (exitCode int, exitMessage string) {
	path := parameters[0]
	givenMode := parameters[1]
	finfo, err := os.Stat(path)
	if err != nil {
		wrkutils.CouldntReadError(path, err)
	}
	actualMode := fmt.Sprint(finfo.Mode().Perm()) // -rwxrw-r-- format
	if actualMode == givenMode {
		return 0, ""
	}
	msg := "File modes did not match"
	return wrkutils.GenericError(msg, givenMode, []string{actualMode})
}

func diskUsage(parameters []string) (exitCode int, exitMessage string) {
	// percentFSUsed gets the percent of the filesystem that is occupied
	percentFSUsed := func(path string) int {
		// get FS info (*nix systems only!)
		var stat syscall.Statfs_t
		syscall.Statfs(path, &stat)

		// blocks * size of block = available size
		totalBytes := stat.Blocks * uint64(stat.Bsize)
		availableBytes := stat.Bavail * uint64(stat.Bsize)
		usedBytes := totalBytes - availableBytes
		percentUsed := int((float64(usedBytes) / float64(totalBytes)) * 100)
		return percentUsed

	}
	maxPercentUsed := wrkutils.ParseMyInt(parameters[1])
	actualPercentUsed := percentFSUsed(parameters[0])
	if actualPercentUsed < maxPercentUsed {
		return 0, ""
	}
	msg := "More disk space used than expected"
	slc := []string{fmt.Sprint(actualPercentUsed) + "%"}
	return wrkutils.GenericError(msg, fmt.Sprint(maxPercentUsed)+"%", slc)
}
