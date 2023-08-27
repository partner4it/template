package template

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	strip "github.com/grokify/html-strip-tags-go"

	xmlToJson "github.com/basgys/goxml2json"
)

//Function used to convert the tempory html file to the pdf
func ToPDF(htmlfile string, pdffile string) error {

	var pdfGrabber = func(url string, sel string, res *[]byte) chromedp.Tasks {
		return chromedp.Tasks{
			emulation.SetUserAgentOverride("WebScraper 1.0"),
			chromedp.Navigate(url),
			// wait for footer element is visible (ie, page is loaded)
			// chromedp.ScrollIntoView(`footer`),
			chromedp.WaitVisible(`body`, chromedp.ByQuery),
			// chromedp.Text(`h1`, &res, chromedp.NodeVisible, chromedp.ByQuery),
			chromedp.ActionFunc(func(ctx context.Context) error {
				buf, _, err := page.PrintToPDF().
					WithDisplayHeaderFooter(true).
					//WithMarginLeft(0).
					//WithMarginRight(0).
					//WithHeaderTemplate(`<div style="font-size:8px;width:100%;text-align:center;"><span class="title"></span> -- <span class="url"></span></div>`).
					WithHeaderTemplate(`<div style="font-size:8px;width:100%;text-align:center;"></div>`).
					WithFooterTemplate(`<div style="font-size:8px;width:100%;text-align:center;margin: 0mm 8mm 0mm 8mm;"><span style="float:left"><span class="title"></span> - <span >` + filepath.Base(pdffile) + `</span></span><span style="float:right">(<span class="pageNumber"></span> / <span class="totalPages"></span>)</span></div>`).
					WithPrintBackground(true).Do(ctx)
				if err != nil {
					return err
				}
				*res = buf
				return nil
			}),
		}
	}

	taskCtx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	var pdfBuffer []byte
	if err := chromedp.Run(taskCtx, pdfGrabber("file://"+htmlfile, "body", &pdfBuffer)); err != nil {
		return err
	}
	if err := ioutil.WriteFile(pdffile, pdfBuffer, 0644); err != nil {
		return err
	}
	return nil
}

//Apply the json data to the template
func ToTemplate(tplName string, data *string) (string, error) {
	return ToTemplateFunc(tplName, data, nil)
}

func ToTemplateFunc(tplName string, data *string, funcMap template.FuncMap) (string, error) {

	//Some handy default functions
	funcs := template.FuncMap{
		"now": time.Now,
		"inc": func(n int) int {
			return n + 1
		},
		"strip": func(html string) string {
			data := strip.StripTags(html)
			data = strings.ReplaceAll(data, "&nbsp;", "")
			return data
		},
		"marshal": func(jsonData ...interface{}) string {
			marshaled, _ := json.MarshalIndent(jsonData[0], "", "   ")
			return string(marshaled)
		},
		"slice": func(args ...interface{}) []interface{} {
			return args
		},
	}

	//Add the funcMap to default funcs
	for k, v := range funcMap {
		funcs[k] = v
	}

	t, err := template.New(filepath.Base(tplName)).Funcs(funcs).ParseFiles(tplName)
	if err != nil {
		return "", err
	}
	tplData := "{\"data\" :" + *data + "}"
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(tplData), &m); err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, m); err != nil {
		return "", err
	}
	tplData = tpl.String()
	return tplData, nil
}

//Convert a xml to PDF using the templatefile and tempfile
func XMLtoPdf(fileIn string, fileOut string, tplName string, tempFile string) error {
	return XMLtoPdfFunc(fileIn, fileOut, tplName, tempFile, nil)
}

func XMLtoPdfFunc(fileIn string, fileOut string, tplName string, tempFile string, funcMap template.FuncMap) error {
	xml, err := os.ReadFile(fileIn)
	if err != nil {
		return err
	}
	// Convert the xml to Json
	json, err := xmlToJson.Convert(strings.NewReader(string(xml)))
	if err != nil {
		return err
	}

	// Make for the bytebuffer a string and run it through the template
	jsonString := json.String()
	content, err := ToTemplateFunc(tplName, &jsonString, funcMap)
	if err != nil {
		return err
	}

	//Save the content to the tempfile
	if err = ioutil.WriteFile(tempFile, []byte(content), 0644); err != nil {
		return err
	}

	// Convert the tempfile to outputfile pdf
	path, err := os.Getwd() //We need fullpath of tempfile
	if err != nil {
		return err
	}
	if err = ToPDF(path+"/"+tempFile, fileOut); err != nil {
		return err
	}
	return nil
}
