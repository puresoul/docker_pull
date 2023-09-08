package cmd

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"go_pull/pkgs/vmconfig"
	"go_pull/pkgs/model"
	"go_pull/pkgs/util/aes"
	"go_pull/pkgs/util/check_path"
	"go_pull/pkgs/util/filetool"
	"go_pull/pkgs/util/logtool"
	"go_pull/pkgs/util/makestr"
	"go_pull/pkgs/util/progress"
	"go_pull/pkgs/util/request"
	"go_pull/pkgs/util/tartool"
	"go_pull/pkgs/util/timetool"

	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
)

var (
	auth_url    string
	reg_service string
	repository  string
	auth_head   map[string]string
	resp        *resty.Response
	err         error
	registry    string
	platform    string
	plist       bool
)

type download_parameter struct {
	layer    interface{}
	layerdir string
	ublob    string
	startbyt int
	endbyt   int
	n        int
	w        *sync.WaitGroup
	progress string
	tfile    *os.File
	err      error
}

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.PersistentFlags().StringVarP(&platform, "platform", "p", "amd64",
		"Select platform system architecture, such as arm64v8, among which v8 is the field variant if it exists")
	downloadCmd.PersistentFlags().BoolVarP(&plist, "show", "s", false, "list platform system architecture")
	downloadCmd.PersistentFlags().IntVarP(&vmconfig.Ptimeout, "timeout", "t", 3, "timeout/s of the request")
	downloadCmd.PersistentFlags().IntVarP(&vmconfig.Piotimeout, "iotimeout", "o", 20, "iotimeout/s of the request")
	downloadCmd.PersistentFlags().IntVarP(&vmconfig.Retry, "retry", "r", 5, "Connection failure is the maximum number of retries")
	downloadCmd.PersistentFlags().StringVarP(&vmconfig.Loglevel, "level", "l", "debug", "log level: debug、info、warn、error")

}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "download image only",
	Args:  cobra.MinimumNArgs(1),
	Long:  `All software has versions. This is pull's`,
	Run: func(cmd *cobra.Command, args []string) {
		logtool.Setloglevel(vmconfig.Loglevel)
		startdownload(args)
	},
}

func get_platform_digest(resp_json map[string]interface{}) (platform_digest string) {
	var platformv_list []string
	manifests, isOk := resp_json["manifests"]
	if !isOk {
		if !plist {

			platform_digest_any, Ok := resp_json["tag"]
			if !Ok {
				logtool.SugLog.Fatal("manifests have no tag key")
			}
			platform_digest = platform_digest_any.(string)
			return
		} else {
			manifests = resp_json
		}
	}

	if plist {
		data, _ := json.MarshalIndent(manifests, "", " ")
		fmt.Println(string(data))
		os.Exit(0)
	}

	for _, v := range manifests.([]interface{}) {

		platformv := v.(map[string]interface{})["platform"]
		platformvStr := platformv.(map[string]interface{})["architecture"].(string)
		if variant, ok := platformv.(map[string]interface{})["variant"]; ok {
			platformvStr = platformvStr + variant.(string)
		}

		if platformvStr == platform {
			platform_digest = v.(map[string]interface{})["digest"].(string)
			break
		}
		platformv_list = append(platformv_list, platformv.(string))

	}
	if platform_digest == "" {
		logtool.SugLog.Fatalf("please use -p %v\n", platformv_list)
	}
	return
}

func startdownload(args []string) {

	// Look for the Docker image to download
	inargv := args
	var img string
	var repo string = "library"
	var tag string = "latest"
	var imlist string
	var digest string
	var imgpartstr string
	registry = "registry-1.docker.io"

	if strings.Contains(inargv[0], "@") {
		s := strings.Split(inargv[0], "@")
		imlist, digest = s[0], s[1]
	} else {
		imlist, digest = inargv[0], ""
	}

	if strings.Contains(inargv[0], ":") {
		s := strings.Split(imlist, ":")
		imgpartstr, tag = s[0], s[1]
	} else {
		imgpartstr, tag = imlist, "latest"
	}

	imgpartlist := strings.Split(imgpartstr, "/")
	img = imgpartlist[len(imgpartlist)-1]

	// Docker client doesn't seem to consider the first element as a potential registry unless there is a '.' or ':'
	if len(imgpartlist) > 1 && (strings.Contains(imgpartlist[0], ".") || strings.Contains(imgpartlist[0], ":")) {
		registry = imgpartlist[0]
		repo = strings.Join(imgpartlist[1:len(imgpartlist)-1], "/")
	} else {
		if len(imgpartlist[:len(imgpartlist)-1]) != 0 {
			repo = strings.Join(imgpartlist[:len(imgpartlist)-1], "/")
		}
	}
	repository = makestr.Joinstring(repo, "/", img)

	//Get Docker authentication endpoint when it is required
	auth_url = "https://auth.docker.io/token"
	reg_service = "registry.docker.io"
	logtool.SugLog.Debug("get docker auth_url...")
	resp, err = request.Requests(
		makestr.Joinstring("https://", registry, "/v2/")).
		Settls().
		Get()
	logtool.Fatalerror(err)
	if resp.StatusCode() == 401 {
		auth_url = resp.Header().Get("Www-Authenticate")
		reg_Header_list := strings.Split(auth_url, "\"")
		auth_url = reg_Header_list[1]
		if len(reg_Header_list) > 4 {
			reg_service = reg_Header_list[3]
		} else {
			reg_service = ""
		}
	}
	//Fetch manifest v2 and get image layer digests
	var platform_digest string

	if digest == "" {
		logtool.SugLog.Debug("get docker auth header...")
		auth_head = get_auth_head("application/vnd.docker.distribution.manifest.list.v2+json")
		queryManifestsListUrl := makestr.Joinstring("https://", registry, "/v2/", repository, "/manifests/", tag)
		resp, err := request.Requests(queryManifestsListUrl).
			Setheads(auth_head).
			Settls().
			Get()
		logtool.Errorerror(err)
		if resp.StatusCode() != 200 {
			logtool.SugLog.Fatal("[-] Cannot fetch manifest for %v [HTTP %v]", repository, resp.Status())
		}
		respJson := request.Parsebody_to_json(resp)

		platform_digest = get_platform_digest(respJson)
	} else {
		platform_digest = digest
	}

	logtool.SugLog.Debug("request again docker auth header if Expired...")
	auth_head = get_auth_head("application/vnd.docker.distribution.manifest.v2+json")
	queryManifestsUrl := makestr.Joinstring("https://", registry, "/v2/", repository, "/manifests/", platform_digest)

	logtool.SugLog.Debug("get docker manifests...")
	resp, err = request.Requests(queryManifestsUrl).
		Setheads(auth_head).
		Settls().
		Get()
	logtool.Fatalerror(err)
	

	rresp := request.Parsebody_to_json(resp)
	layers := rresp["layers"].([]interface{})

	//Create tmp folder that will hold the image
	imgdir := makestr.Joinstring("tmp_", img, "_", strings.ReplaceAll(tag, "@", ""))
	if check_path.Check_path(imgdir).Exists() {
		os.RemoveAll(imgdir)
	}
	os.Mkdir(imgdir, os.ModePerm)
	logtool.SugLog.Infof("Creating image structure in: %v", imgdir)

	config := rresp["config"].(map[string]interface{})["digest"].(string)

	logtool.SugLog.Debug("get docker blobs config...")
	confresp, err := request.Requests(
		makestr.Joinstring("https://", registry, "/v2/", repository, "/blobs/", config)).
		Setheads(auth_head).
		Settls().
		Get()
	logtool.Fatalerror(err)
	f := filetool.GetfileOjb(makestr.Joinstring(imgdir, "/", config[7:], ".json"))
	f.WriteString(confresp.String())
	f.Close()

	content := model.Contentvar()
	content[0].Config = makestr.Joinstring(config[7:], ".json")

	if len(imgpartlist[:len(imgpartlist)-1]) != 0 {
		content[0].RepoTags = append(
			content[0].RepoTags,
			makestr.Joinstring(strings.Join(imgpartlist[:len(imgpartlist)-1], "/"), "/", img, ":", tag),
		)
	} else {
		content[0].RepoTags = append(
			content[0].RepoTags,
			makestr.Joinstring(img, ":", tag),
		)
	}

	//Build layer folders
	var wg sync.WaitGroup
	wg = sync.WaitGroup{}
	wg.Add(len(layers))

	var parentid string
	var last_fake_layerid string
	logtool.SugLog.Debug("Start concurrent downloads...")
	for x, layer := range layers {
		ublob := layer.(map[string]interface{})["digest"].(string)
		logtool.SugLog.Info(ublob)
		fake_layerid := aes.Sha256t(makestr.Joinstring(parentid, "\n", ublob, "\n"))
		layerdir := makestr.Joinstring(imgdir, "/", fake_layerid)
		os.Mkdir(layerdir, os.ModePerm)
		//Creating VERSION file
		vf := filetool.GetfileOjb(makestr.Joinstring(layerdir, "/VERSION"))
		vf.WriteString("1.0")
		vf.Close()

		//layer interface{}, layerdir string, ublob string, length string, w *sync.WaitGroup

		go Download_img(download_parameter{
			layer:    layer,
			layerdir: layerdir,
			ublob:    ublob,
			startbyt: 0,
			n:        0,
			w:        &wg,
		})

		content[0].Layers = append(content[0].Layers, makestr.Joinstring(fake_layerid, "/layer.tar"))
		//Creating json file
		f2 := filetool.GetfileOjb(makestr.Joinstring(layerdir, "/json"))
		//last layer = config manifest - history - rootfs
		var json_obj map[string]interface{}
		if layers[len(layers)-1].(map[string]interface{})["digest"].(string) ==
			layer.(map[string]interface{})["digest"].(string) {
			json_obj = request.Parsebody_to_json(confresp)
			delete(json_obj, "history")
			if _, ok := json_obj["rootfs"]; ok {
				//存在
				delete(json_obj, "rootfs")
			} else if _, ok := json_obj["rootfS"]; ok {
				delete(json_obj, "rootfS")
			}
		} else {
			json_obj = model.Empty_config()
		}
		json_obj["id"] = fake_layerid

		if parentid != "" {
			json_obj["parent"] = parentid
		}
		parentid = fake_layerid
		data, _ := json.Marshal(json_obj)
		f2.Write(data)
		f2.Close()

		if x+1 == len(layers) {
			last_fake_layerid = fake_layerid
		}

	}
	wg.Wait()

	f3 := filetool.GetfileOjb(makestr.Joinstring(imgdir, "/manifest.json"))
	data, _ := json.Marshal(content)
	f3.Write(data)
	f3.Close()

	var content1 map[string](map[string]string)
	if len(imgpartlist[:len(imgpartlist)-1]) != 0 {

		content1 = map[string](map[string]string){
			makestr.Joinstring(strings.Join(imgpartlist[:len(imgpartlist)-1], "/"), img): map[string]string{tag: last_fake_layerid},
		}
	} else {
		content1 = map[string](map[string]string){
			img: map[string]string{tag: last_fake_layerid},
		}

	}

	f5 := filetool.GetfileOjb(makestr.Joinstring(imgdir, "/repositories"))
	data1, _ := json.Marshal(content1)
	f5.Write(data1)
	f5.Close()

	//Create image tar and clean tmp folder
	//docker_tar:= makestr.Joinstring(
	//strings.ReplaceAll(repo,"/", "_"),
	//"_",img,".tar")
	fmt.Print("Creating archive...")
	os.Stdout.Sync()

	if check_path.Check_path(img + ".tar").Exists() {
		os.Remove(img + ".tar")
	}
	tartool.Tar(img+".tar", imgdir)
	os.RemoveAll(imgdir)
	fmt.Printf("打包完成，生成文件 %v\n", img+".tar")
}

func check_head(Header http.Header) bool {
	if Header.Get("Accept-Ranges") == "bytes" {
		return true
	}
	return false
}

// func Download_img(layer interface{}, layerdir string, ublob string, length string, w *sync.WaitGroup) {
func Download_img(parameter download_parameter) {
	if parameter.n == 0 {
		parameter.tfile = filetool.GetfileOjb(makestr.Joinstring(parameter.layerdir, "/layer.tar.gz"))
		logtool.SugLog.Infof("%v%v", parameter.ublob[7:19], ": Downloading...")
	} else if parameter.n < 5 {
		logtool.SugLog.Infof("%v%v", parameter.ublob[7:19], ": try to download again...")
	}

	parameter.n += 1
	// Creating layer.tar file
	os.Stdout.Sync()
	auth_head := get_auth_head("application/vnd.docker.distribution.manifest.v2+json", auth_head)
	bresp, err := request.Requests(
		makestr.Joinstring("https://", registry, "/v2/", repository, "/blobs/", parameter.ublob)).
		Notparse().
		Setheads(auth_head).
		Setheads(map[string]string{"Range": "bytes=" + strconv.Itoa(parameter.startbyt) + "-"}).
		Settls().
		Get()

	defer func() {
		if parameter.progress == "" {
			return
		}
		parameter.tfile.Close()
		os.Remove(parameter.tfile.Name())
		if parameter.progress == "done" {
			fmt.Printf("%v: Extracting...%v", parameter.ublob[7:19], strings.Repeat(" ", 50))
			os.Stdout.Sync()

			fmt.Printf("%v: Pull complete \n",
				parameter.ublob[7:19])
			(*parameter.w).Done()
		}
		if parameter.err != nil {
			logtool.SugLog.Fatal(parameter.err)
		}
	}()

	if err != nil {
		parameter.progress = "err"
		parameter.err = err
		return
	}

	if bresp.StatusCode() != 206 && bresp.StatusCode() != 200 {
		logtool.SugLog.Warn(parameter.layer.(map[string]interface{}))
		urls, ok := parameter.layer.(map[string]interface{})["urls"]
		if ok {
			bresp, _ := request.Requests(urls.([]string)[0]).
				Setheads(auth_head).
				Settls().
				Get()
			if bresp.StatusCode() != 206 && bresp.StatusCode() != 200 {
				fmt.Printf("\rERROR: Cannot download layer %v [HTTP %v %v]", parameter.ublob[7:19], bresp.StatusCode(), bresp.Header()["Content-Length"])
				logtool.SugLog.Fatal(bresp)
			}
		} else {
			logtool.SugLog.Fatalf("no url to download layer %v", parameter.ublob[7:19])
		}
	}

	//Stream download and follow the progress
	totalLength, _ := strconv.Atoi(bresp.Header().Get("Content-Length"))
	buf := make([]byte, 65536)

	teeReader := io.TeeReader(bresp.RawBody(),
		&progress.Progress{
			Ublob:             parameter.ublob[7:19],
			Total:             totalLength,
			ProgressBarLength: 50,
		},
	)
	if err != nil {
		logtool.SugLog.Fatal(err)
	}
	for {
		n, err := teeReader.Read(buf)

		parameter.startbyt = parameter.startbyt + n
		parameter.tfile.Write(buf[:n])
		if err != nil {
			if err == io.EOF {
				fmt.Println("")
				parameter.progress = "done"
				fmt.Printf("%v: wait write to file...%v\n", parameter.ublob[7:19], strings.Repeat(" ", 50))
				_, err = parameter.tfile.Seek(0, 0)
				logtool.Fatalerror(err)
				tarFile := filetool.GetfileOjb(makestr.Joinstring(parameter.layerdir, "/layer.tar"))
				greader, err := gzip.NewReader(parameter.tfile)
				defer greader.Close()
				_, err = io.Copy(tarFile, greader)
				logtool.Fatalerror(err)
			} else {
				logtool.SugLog.Warn(err, " ioerr")

				if parameter.n < 5 {
					Download_img(parameter)
				} else {
					parameter.progress = "err"
					parameter.err = errors.New("ioerr: reconnecting more than 5 times")
				}
			}
			break
		}
	}
}

// Get Docker token (this function is useless for unauthenticated registries like Microsoft)
func get_auth_head(qtype string, a ...any) map[string]string {
	if len(a) != 0 {
		t := a[0].(map[string]string)
		tm := t["expires_in"]
		st := timetool.Strtorime(tm, "UTC")
		if st.Add(-2 * time.Second).After(time.Now().UTC()) {
			t["Accept"] = qtype
			return t
		}

	}
	resp, err := request.Requests(
		makestr.Joinstring(auth_url, "?service=", reg_service, "&scope=repository:", repository, ":pull")).
		Settls().
		Get()
	logtool.Fatalerror(err)
	resp_json := request.Parsebody_to_json(resp)
	expires_in := int(resp_json["expires_in"].(float64))
	issued_at := resp_json["issued_at"].(string)

	expires_time := timetool.Strtorime(issued_at, "UTC").
		Add(time.Duration(expires_in) * (time.Hour)).
		Format("2006-01-02 15:04:05")

	auth_head := map[string]string{"Authorization": makestr.Joinstring("Bearer ", resp_json["token"].(string)),
		"Accept":     qtype,
		"expires_in": expires_time,
	}
	return auth_head

}
