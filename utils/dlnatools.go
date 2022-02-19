package utils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/h2non/filetype"
)

var (
	// If we're looking to use the dlnaOrgFlagSenderPaced
	// flag for a 32bit build, we need to make sure that we
	// first convert all the flag types to int64
	//dlnaOrgFlagSenderPaced = 1 << 31
	//dlnaOrgFlagTimeBasedSeek = 1 << 30
	//dlnaOrgFlagByteBasedSeek = 1 << 29
	//dlnaOrgFlagPlayContainer = 1 << 28
	//dlnaOrgFlagS0Increase = 1 << 27
	//dlnaOrgFlagSnIncrease = 1 << 26
	//dlnaOrgFlagRtspPause = 1 << 25
	dlnaOrgFlagStreamingTransferMode = 1 << 24
	//dlnaOrgFlagInteractiveTransfertMode = 1 << 23
	dlnaOrgFlagBackgroundTransfertMode = 1 << 22
	dlnaOrgFlagConnectionStall         = 1 << 21
	dlnaOrgFlagDlnaV15                 = 1 << 20

	dlnaprofiles = map[string]string{
		"video/x-mkv":             "DLNA.ORG_PN=MATROSKA",
		"video/x-matroska":        "DLNA.ORG_PN=MATROSKA",
		"video/x-msvideo":         "DLNA.ORG_PN=AVI",
		"video/mpeg":              "DLNA.ORG_PN=MPEG1",
		"video/vnd.dlna.mpeg-tts": "DLNA.ORG_PN=MPEG1",
		"video/mp4":               "DLNA.ORG_PN=AVC_MP4_MP_SD_AAC_MULT5",
		"video/quicktime":         "DLNA.ORG_PN=AVC_MP4_MP_SD_AAC_MULT5",
		"video/x-m4v":             "DLNA.ORG_PN=AVC_MP4_MP_SD_AAC_MULT5",
		"video/3gpp":              "DLNA.ORG_PN=AVC_MP4_MP_SD_AAC_MULT5",
		"video/x-flv":             "DLNA.ORG_PN=AVC_MP4_MP_SD_AAC_MULT5",
		"audio/mpeg":              "DLNA.ORG_PN=MP3",
		"image/jpeg":              "JPEG_LRG",
		"image/png":               "PNG_LRG"}
)

func defaultStreamingFlags() string {
	return fmt.Sprintf("%.8x%.24x", dlnaOrgFlagStreamingTransferMode|
		dlnaOrgFlagBackgroundTransfertMode|
		dlnaOrgFlagConnectionStall|
		dlnaOrgFlagDlnaV15, 0)
}

// BuildContentFeatures - Build the content features string
// for the "contentFeatures.dlna.org" header.
func BuildContentFeatures(mediaType string, seek string, transcode bool) (string, error) {
	var cf strings.Builder

	if mediaType != "" {
		dlnaProf, profExists := dlnaprofiles[mediaType]
		switch profExists {
		case true:
			cf.WriteString(dlnaProf + ";")
		default:
			return "", errors.New("non supported mediaType")
		}
	}

	// "00" neither time seek range nor range supported
	// "01" range supported
	// "10" time seek range supported
	// "11" both time seek range and range supported
	switch seek {
	case "00":
		cf.WriteString("DLNA.ORG_OP=00;")
	case "01":
		cf.WriteString("DLNA.ORG_OP=01;")
	case "10":
		cf.WriteString("DLNA.ORG_OP=10;")
	case "11":
		cf.WriteString("DLNA.ORG_OP=11;")
	default:
		return "", errors.New("invalid seek flag")
	}

	switch transcode {
	case true:
		cf.WriteString("DLNA.ORG_CI=1;")
	default:
		cf.WriteString("DLNA.ORG_CI=0;")
	}

	cf.WriteString("DLNA.ORG_FLAGS=")
	cf.WriteString(defaultStreamingFlags())

	return cf.String(), nil
}

// GetMimeDetailsFromFile - Get media file mime details.
func GetMimeDetailsFromFile(f string) (string, error) {
	file, err := os.Open(f)
	if err != nil {
		return "", fmt.Errorf("getMimeDetailsFromFile error: %w", err)
	}
	defer file.Close()

	head := make([]byte, 261)
	file.Read(head)

	kind, err := filetype.Match(head)
	if err != nil {
		return "", fmt.Errorf("getMimeDetailsFromFile error #2: %w", err)
	}

	return fmt.Sprintf("%s/%s", kind.MIME.Type, kind.MIME.Subtype), nil
}

// GetMimeDetailsFromStream - Get media URL mime details.
func GetMimeDetailsFromStream(s io.ReadCloser) (string, error) {
	defer s.Close()
	head := make([]byte, 261)
	s.Read(head)

	kind, err := filetype.Match(head)
	if err != nil {
		return "", fmt.Errorf("getMimeDetailsFromStream error: %w", err)
	}

	return fmt.Sprintf("%s/%s", kind.MIME.Type, kind.MIME.Subtype), nil
}
