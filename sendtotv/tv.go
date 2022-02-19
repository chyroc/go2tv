package sendtotv

import (
	_ "embed"
	"io"
	"time"

	"github.com/chyroc/go2tv/httphandlers"
	"github.com/chyroc/go2tv/interactive"
	"github.com/chyroc/go2tv/soapcalls"
	"github.com/chyroc/go2tv/utils"
)

type Media struct {
	Name string
	Body io.ReadCloser
}

func SendReadCloser(media, subTitle *Media, dmrURL string) error {
	mediaBody := media.Body
	mediaName := media.Name
	subTitleName := ""
	var subTitleBody io.ReadCloser
	if subTitle != nil {
		subTitleName = subTitle.Name
		subTitleBody = subTitle.Body
	}
	mediaType := ""

	upnpServicesURLs, err := soapcalls.DMRextractor(dmrURL)
	if err != nil {
		return err
	}

	whereToListen, err := utils.URLtoListenIPandPort(dmrURL)
	if err != nil {
		return err
	}

	scr, err := interactive.InitTcellNewScreen()
	if err != nil {
		return err
	}

	callbackPath, err := utils.RandomString()
	if err != nil {
		return err
	}

	tvdata := &soapcalls.TVPayload{
		ControlURL:          upnpServicesURLs.AvtransportControlURL,
		EventURL:            upnpServicesURLs.AvtransportEventSubURL,
		RenderingControlURL: upnpServicesURLs.RenderingControlURL,
		CallbackURL:         "http://" + whereToListen + "/" + callbackPath,
		MediaURL:            "http://" + whereToListen + "/" + utils.ConvertFilename(mediaName),
		SubtitlesURL:        "http://" + whereToListen + "/" + utils.ConvertFilename(subTitleName),
		MediaType:           mediaType,
		CurrentTimers:       make(map[string]*time.Timer),
	}

	s := httphandlers.NewServer(whereToListen)
	serverStarted := make(chan struct{})

	// We pass the tvdata here as we need the callback handlers to be able to react
	// to the different media renderer states.
	var finalErr error
	go func() {
		finalErr = s.ServeFiles(serverStarted, mediaBody, subTitleBody, tvdata, scr)
	}()
	// Wait for HTTP server to properly initialize
	<-serverStarted

	if err = scr.InterInit(tvdata); err != nil {
		return err
	}

	return finalErr
}
