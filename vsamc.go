package main

import (
	"bufio"
	"fmt"
	"github.com/fhs/gompd/mpd"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var conn *mpd.Client

var browserWinid int
var playlistWinid int

var browserBodyFile *os.File
var playlistBodyFile *os.File

var currentPath string
var quit bool

type event struct {
	middlemouse bool
	text        string
}

// this is the limit for the text that appears in event file (see acme(4))
const eventtextlimit = 256

func createMpdConn() *mpd.Client {
	mpdHost, hostEnvSet := os.LookupEnv("MPD_HOST")
	mpdPort, portEnvSet := os.LookupEnv("MPD_PORT")
	mpdPassword := ""

	// extract password of MPD_HOST
	if hostEnvSet {
		if strings.Contains(mpdHost, "@") {
			mpdPassword = strings.Split(mpdHost, "@")[0]
			mpdHost = strings.Split(mpdHost, "@")[1]
		}
	} else {
		mpdHost = "localhost"
	}

	// default to mpd's default port
	if !portEnvSet {
		mpdPort = "6600"
	}

	var err error

	if mpdPassword == "" {
		conn, err = mpd.Dial("tcp", mpdHost+":"+mpdPort)
	} else {
		conn, err = mpd.DialAuthenticated("tcp", mpdHost+":"+mpdPort, mpdPassword)
	}
	if err != nil {
		panic(err)
	}
	return conn
}

func bodyLength(winid int) int {
	dat, err := ioutil.ReadFile(fmt.Sprintf("/mnt/acme/%d/ctl", winid))
	if err != nil {
		panic(err)
	}

	ctlString := strings.TrimSpace(string(dat))

	i, err := strconv.Atoi(strings.Fields(ctlString)[2])
	if err != nil {
		panic(err)
	}

	return i
}

func clearBody(winid int, file *os.File) {
	len := bodyLength(winid)
	file.WriteString(strings.Repeat("\b", len))
}

func createWindow() int {
	dat, err := ioutil.ReadFile("/mnt/acme/new/ctl")
	if err != nil {
		panic(err)
	}

	ctlString := strings.TrimSpace(string(dat))

	i, err := strconv.Atoi(strings.Fields(ctlString)[0])
	if err != nil {
		panic(err)
	}

	return i
}

func deleteWindow(winid int) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/ctl", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString("delete\n")
	file.Close()
}

func parseEvent(eventStr string) (event, bool) {
	parsed := false
	evt := event{false, "foo"}

	if eventStr != "" && eventStr[0] == 'M' {
		if eventStr[1] == 'x' || eventStr[1] == 'X' {
			evt.middlemouse = true
			out := strings.SplitAfterN(eventStr, " ", 5)
			evt.text = strings.Trim(out[4], " ")
			parsed = evt.text != ""
		} else if eventStr[1] == 'l' || eventStr[1] == 'L' {
			evt.middlemouse = false
			out := strings.SplitAfterN(eventStr, " ", 5)
			evt.text = strings.Trim(out[4], " ")
			parsed = evt.text != ""
		}
	}

	return evt, parsed
}

func mpdClosedConn(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "EOF") ||
		strings.Contains(err.Error(), "Hangup") ||
		strings.Contains(err.Error(), "broken pipe")

}

func setDataAddr(winid int, addr string) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/addr", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString(addr)
	file.Close()
}

func writeName(winid int, name string) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/ctl", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString(fmt.Sprintf("name %s\n", name))
	file.Close()
}

func writeTags(winid int, tags string) {
	file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/tag", winid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	file.WriteString(tags)
	file.Close()
}

func absPathFromRelPath(wd string, relPath string) string {
	if relPath == "" || relPath == "." {
		return wd
	}

	if relPath == ".." {
		parentPath := strings.TrimRight(wd, " /")
		// if no parent exists (e.g. wd is "/" do not do anything)
		if parentPath == "" {
			return wd
		}
		parentPath = parentPath[:strings.LastIndex(parentPath, "/")]
		return parentPath
	}

	if relPath[0] == '/' {
		return relPath
	} else {
		return strings.TrimRight(wd, " /") + "/" + strings.Trim(relPath, "/ ")
	}
}

func relPathFromAbsPath(absPath string) string {
	path := strings.Trim(absPath, "/")
	slices := strings.Split(path, "/")
	return slices[len(slices)-1]
}

func convertForPrint(path string) string {
	s := path
	s = strings.Replace(s, "(", "〔", -1)
	s = strings.Replace(s, ")", "〕", -1)
	s = strings.Replace(s, "&", "⊕", -1)
	s = strings.Replace(s, "?", "¿", -1)
	s = strings.Replace(s, "'", "´", -1)
	s = strings.Replace(s, "[", "【", -1)
	s = strings.Replace(s, "]", "】", -1)
	s = strings.Replace(s, ":", "᛬", -1)
	s = strings.Replace(s, "<", "〈", -1)
	s = strings.Replace(s, ">", "〉", -1)
	s = strings.Replace(s, "+", "±", -1)
	s = strings.Replace(s, ".", "。", -1)
	s = strings.Replace(s, "-", "‒", -1)
	s = strings.Replace(s, ",", "、", -1)
	s = strings.Replace(s, " ", "⋯", -1)
	s = strings.Replace(s, "!", "¡", -1)
	s = strings.Replace(s, "#", "﹟", -1)
	s = strings.Replace(s, "{", "﹛", -1)
	s = strings.Replace(s, "}", "﹜", -1)
	return s
}

func convertFromPrint(s string) string {
	path := s
	path = strings.Replace(path, "〔", "(", -1)
	path = strings.Replace(path, "〕", ")", -1)
	path = strings.Replace(path, "⊕", "&", -1)
	path = strings.Replace(path, "¿", "?", -1)
	path = strings.Replace(path, "´", "'", -1)
	path = strings.Replace(path, "【", "[", -1)
	path = strings.Replace(path, "】", "]", -1)
	path = strings.Replace(path, "᛬", ":", -1)
	path = strings.Replace(path, "〈", "<", -1)
	path = strings.Replace(path, "〉", ">", -1)
	path = strings.Replace(path, "±", "+", -1)
	path = strings.Replace(path, "。", ".", -1)
	path = strings.Replace(path, "‒", "-", -1)
	path = strings.Replace(path, "、", ",", -1)
	path = strings.Replace(path, "⋯", " ", -1)
	path = strings.Replace(path, "¡", "!", -1)
	path = strings.Replace(path, "﹟", "#", -1)
	path = strings.Replace(path, "﹛", "{", -1)
	path = strings.Replace(path, "﹜", "}", -1)
	return path
}

func showPathInBrowser(uri string) bool {
	trimmedPath := strings.Trim(uri, "/")
	attrs, err := conn.ListInfo(trimmedPath)
	if err != nil {
		if mpdClosedConn(err) {
			conn = createMpdConn()
			attrs, err = conn.ListInfo(trimmedPath)
			if err != nil {
				return false
			}
		} else {
			return false
		}
	}

	clearBody(browserWinid, browserBodyFile)
	browserBodyFile.WriteString(fmt.Sprintf("current path: /%s\n", convertForPrint(trimmedPath)))
	for _, attr := range attrs {
		dir := attr["directory"]
		file := attr["file"]
		if dir != "" {
			browserBodyFile.WriteString(fmt.Sprintf("%s\n", convertForPrint(relPathFromAbsPath(dir))))
		} else {
			browserBodyFile.WriteString(fmt.Sprintf("%s\n", convertForPrint(relPathFromAbsPath(file))))
		}
	}
	return true
}

func showPlaylist() {
	attrs, err := conn.PlaylistInfo(-1, -1)
	if err != nil {
		if mpdClosedConn(err) {
			conn = createMpdConn()
			attrs, err = conn.PlaylistInfo(-1, -1)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			fmt.Println(err.Error())
		}
	}

	total := 0
	for _, attr := range attrs {
		artist := attr["Artist"]
		title := attr["Title"]
		duration, _ := strconv.Atoi(attr["Time"])
		total += duration
		minutes := duration / 60
		seconds := duration % 60
		if artist == "" || title == "" {
			playlistBodyFile.WriteString(fmt.Sprintf("# %s # %s # %02d:%02d\n", attr["Pos"], attr["file"], minutes, seconds))
		} else {
			playlistBodyFile.WriteString(fmt.Sprintf("# %s # %s - %s # %02d:%02d\n", attr["Pos"], attr["Artist"], attr["Title"], minutes, seconds))
		}
	}
	playlistBodyFile.WriteString(fmt.Sprintf("TOTAL: %d:%02d\n", total/60, total%60))
}

func showStatus(refresh bool) {
	if refresh {
		playlistBodyFile.WriteString("\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b")
	}
	attrs, err := conn.Status()
	if err != nil {
		if mpdClosedConn(err) {
			conn = createMpdConn()
			attrs, err = conn.Status()
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	playlistBodyFile.WriteString(fmt.Sprintf("State: %-5s Song: %-5s Time: %-20s\n", attrs["state"], attrs["song"], attrs["time"]))
}

func readPlaylistEvents() {
	file, err := os.Open(fmt.Sprintf("/mnt/acme/%d/event", playlistWinid))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for !quit && scanner.Scan() {
		evt, parsed := parseEvent(scanner.Text())
		if parsed {
			handlePlaylistEvent(evt)
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func handlePlaylistEvent(evt event) {
	if evt.middlemouse {
		switch evt.text {
		case "Quit":
			quit = true
			deleteWindow(playlistWinid)
			closeBrowser()
			os.Exit(0)
		case "Play":
			err := conn.Play(-1)
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Play(-1)
			}
			refresh(false)
		case "Stop":
			err := conn.Stop()
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Stop()
			}
			refresh(false)
		case "Pause":
			err := conn.Pause(true)
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Pause(true)
			}
			refresh(false)
		case "Next":
			err := conn.Next()
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Next()
			}
			refresh(false)
		case "Clear":
			err := conn.Clear()
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Clear()
			}
			refresh(true)
		case "Shuffle":
			err := conn.Shuffle(-1, -1)
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Shuffle(-1, -1)
			}
			refresh(true)
		case "Consume":
			err := conn.Consume(true)
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Consume(true)
			}
			refresh(false)
		case "NoConsume":
			err := conn.Consume(false)
			if mpdClosedConn(err) {
				conn = createMpdConn()
				conn.Consume(false)
			}
			refresh(false)
		case "Refresh":
			refresh(true)
		case "Browse":
			if browserWinid < 0 {
				createNewBrowser("/")
			} else {
				currentPath = "/"
				showPathInBrowser(currentPath)
			}
		default:
			if strings.HasPrefix(evt.text, "Move") {
				slices := strings.Fields(evt.text)
				if len(slices) != 3 {
					return // malformed Move <from> <to> command
				}
				i, err := strconv.Atoi(slices[1])
				if err == nil {
					u, err := strconv.Atoi(slices[2])
					if err == nil {
						err = conn.Move(i, -1, u)
						if mpdClosedConn(err) {
							conn = createMpdConn()
							conn.Move(i, -1, u)
						}
						refresh(true)
					}
				}
			} else if strings.HasPrefix(evt.text, "Del") {
				slices := strings.Fields(evt.text)
				if len(slices) != 2 {
					return // malformed Del <position>
				}
				i, err := strconv.Atoi(slices[1])
				if err == nil {
					err = conn.Delete(i, -1)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Delete(i, -1)
					}
					refresh(true)
				}
			} else if strings.HasPrefix(evt.text, "rDel") {
				slices := strings.Fields(evt.text)
				if len(slices) != 3 {
					return // malformed rDel <from> <to> command
				}
				i, err := strconv.Atoi(slices[1])
				if err == nil {
					u, err := strconv.Atoi(slices[2])
					if err == nil {
						err = conn.Delete(i, u)
						if mpdClosedConn(err) {
							conn = createMpdConn()
							conn.Delete(i, u)
						}
					}
					refresh(true)
				}
			} else {
				i, err := strconv.Atoi(evt.text)
				if err == nil {
					err = conn.Play(i)
					if mpdClosedConn(err) {
						conn = createMpdConn()
						conn.Play(i)
					}
					refresh(false)
				}
			}
		}
	} else {
		i, err := strconv.Atoi(evt.text)
		if err == nil {
			attrs, err := conn.PlaylistInfo(i, -1)
			if err != nil {
				if mpdClosedConn(err) {
					conn = createMpdConn()
					attrs, err = conn.PlaylistInfo(i, -1)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					fmt.Println(err.Error())
				}
			}

			slices := strings.Split(attrs[0]["file"], "/")
			filePath := ""
			for i := 0; i < (len(slices) - 1); i++ {
				filePath += "/" + slices[i]
			}
			if browserWinid < 0 {
				createNewBrowser(filePath)
			} else {
				currentPath = filePath
				showPathInBrowser(currentPath)
			}
		}
	}
}

func closeBrowser() {
	if browserWinid > -1 {
		browserBodyFile.Close()
		deleteWindow(browserWinid)
		browserWinid = -1
	}
}

func createNewBrowser(filePath string) {
	var err error
	browserWinid = createWindow()

	browserBodyFile, err = os.OpenFile(fmt.Sprintf("/mnt/acme/%d/body", browserWinid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	writeName(browserWinid, "browse:")
	writeTags(browserWinid, "Close Update Info ..")
	if showPathInBrowser(filePath) {
		currentPath = filePath
	}
	go readBrowserEvents()
}

func readBrowserEvents() {
	fieldsRead := 0
	field := ""
	ismouseevt := false
	evt := event{false, ""}

	// needed for getting runes from data file (if event text >= eventtextlimit)
	startaddr := -1
	endaddr := -1

	file, err := os.Open(fmt.Sprintf("/mnt/acme/%d/event", browserWinid))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	eventReader := bufio.NewReader(file)
	for browserWinid > 0 {
		rune, _, err := eventReader.ReadRune()

		if err != nil {
			panic(err)
		}

		runestr := string(rune)
		switch runestr {
		case "\n":
			field = ""
			fieldsRead = 0
			ismouseevt = false
			evt.text = ""
			startaddr = -1
			endaddr = -1
		case " ":
			fieldsRead++
			if fieldsRead == 1 {
				if len(field) > 2 {
					if field[0] == 'M' {
						if field[1] == 'x' || field[1] == 'X' {
							evt.middlemouse = true
							ismouseevt = true
						} else if field[1] == 'l' || field[1] == 'L' {
							evt.middlemouse = false
							ismouseevt = true
						}
					}

					if ismouseevt {
						startaddr, err = strconv.Atoi(field[2:])
						if err != nil {
							startaddr = -1
						}
					}
				}
			} else if fieldsRead == 2 {
				endaddr, err = strconv.Atoi(field)
				if err != nil {
					endaddr = -1
				}
			} else if ismouseevt && fieldsRead == 4 {
				utflen, err := strconv.Atoi(field)
				if err == nil {
					if utflen > 0 {
						for i := 0; i < utflen; i++ {
							rune, _, err := eventReader.ReadRune()
							if err != nil {
								panic(err)
							}

							evt.text += string(rune)
						}
						handleBrowserEvent(evt)
					} else if utflen == 0 {
						runecount := endaddr - startaddr
						if runecount < eventtextlimit || endaddr < 0 || startaddr < 0 {
							continue
						}
						evt.text = runesFromDataFile(browserWinid, startaddr, endaddr)
						handleBrowserEvent(evt)
					}
				}
			}
			field = ""
		default:
			field += runestr
		}
	}
}

func runesFromDataFile(winid int, start int, end int) string {
	runes := ""
	count := end - start
	setDataAddr(browserWinid, fmt.Sprintf("#%d", start))

	datafile, err := os.Open(fmt.Sprintf("/mnt/acme/%d/data", winid))
	if err != nil {
		panic(err)
	}
	defer datafile.Close()

	r := bufio.NewReader(datafile)

	for i := 0; i < count; i++ {
		rune, _, err := r.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}

		runes += string(rune)
	}
	return runes
}

func handleBrowserEvent(evt event) {
	evt.text = convertFromPrint(evt.text)
	if evt.middlemouse {
		switch evt.text {
		case "Close":
			closeBrowser()
			return
		case "Update":
			path := strings.Trim(currentPath, " /")
			_, err := conn.Update(path)
			fmt.Println("Updating")
			if mpdClosedConn(err) {
				conn = createMpdConn()
				_, err = conn.Update(path)
			}

			if err != nil {
				fmt.Fprintf(os.Stderr, "Failure to update path '%s': %s\n", path, err.Error())
			}
		default:
			if strings.HasPrefix(evt.text, "Info") {
				slices := strings.Fields(evt.text)
				if len(slices) < 2 {
					return // malformed Info <filePath>
				}
				relPath := ""
				for i := 1; i < len(slices); i++ {
					relPath += " " + slices[i]
				}
				filePath := strings.Trim(absPathFromRelPath(currentPath, relPath), " /")
				cmd := exec.Command("songinfo", filePath)
				newbrowserWinid := createWindow()
				file, err := os.OpenFile(fmt.Sprintf("/mnt/acme/%d/body", newbrowserWinid), os.O_APPEND|os.O_WRONLY, 0600)
				writeName(newbrowserWinid, "/tmp/songinfo")
				writeTags(newbrowserWinid, "Delete")
				if err != nil {
					panic(err)
				}
				cmd.Stdout = file
				err = cmd.Start()
				if err != nil {
					fmt.Println(err)
				}
				cmd.Wait()
				file.Close()
			} else {
				var err error
				relFilePaths := strings.Split(evt.text, "\n")
				for i := 0; i < len(relFilePaths); i++ {
					filePath := strings.Trim(absPathFromRelPath(currentPath, relFilePaths[i]), " /")
					err = conn.Add(filePath)
					if i == 0 && mpdClosedConn(err) {
						conn = createMpdConn()
						err = conn.Add(filePath)
					}
				}
				if err == nil {
					refresh(true)
				}
			}
		}
	} else {
		newPath := absPathFromRelPath(currentPath, strings.Trim(evt.text, " "))
		// only change path on success
		if showPathInBrowser(newPath) {
			currentPath = newPath
		}
	}
}

func refresh(full bool) {
	if full {
		clearBody(playlistWinid, playlistBodyFile)
		showPlaylist()
	}
	showStatus(!full)
}

func main() {
	var err error

	browserWinid = -1
	quit = false
	conn = createMpdConn()
	defer conn.Close()

	playlistWinid = createWindow()

	playlistBodyFile, err = os.OpenFile(fmt.Sprintf("/mnt/acme/%d/body", playlistWinid), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	writeName(playlistWinid, "samc:")
	writeTags(playlistWinid, "Quit Clear Play Pause Stop Next Browse Refresh")

	showPlaylist()
	showStatus(false)

	readPlaylistEvents()

	playlistBodyFile.Close()
}
