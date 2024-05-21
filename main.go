package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

/*
#include <windows.h>

typedef struct monitor {
	LONG x, y, w, h;
	WCHAR name[CCHDEVICENAME];
} monitor;

typedef struct enumData {
	LONG numMonitors;
	monitor monitors[256];
	DWORD failCode;
	BOOL failed;
} enumData;

BOOL monitorEnumProc(HMONITOR hMonitor, HDC hdc, LPRECT lpRect, LPARAM lParam) {
	enumData* d = (enumData*)lParam;
	MONITORINFOEXW mi;
	mi.cbSize = sizeof(MONITORINFOEXW);
	BOOL success = GetMonitorInfoW(hMonitor, (void*)&mi);
	if (!success) {
		d->failCode = GetLastError();
		d->failed = TRUE;
		return FALSE;
	}
	d->monitors[d->numMonitors].x = mi.rcMonitor.left;
	d->monitors[d->numMonitors].y = mi.rcMonitor.top;
	d->monitors[d->numMonitors].w = mi.rcMonitor.right - mi.rcMonitor.left;
	d->monitors[d->numMonitors].h = mi.rcMonitor.bottom - mi.rcMonitor.top;
	memcpy(&d->monitors[d->numMonitors].name[0], &mi.szDevice[0], sizeof(WCHAR)*CCHDEVICENAME);
	mi.szDevice[CCHDEVICENAME-1] = (WCHAR)0;
	d->numMonitors++;
	return d->numMonitors < 256L;
}
*/
import "C"

const maxMonitors = 256

type cMonitor struct {
	x, y, w, h int32
	name       [C.CCHDEVICENAME]C.WCHAR
}

type cMonitorEnumData struct {
	numMonitors int32
	monitors    [maxMonitors]cMonitor
	failCode    uint32
	failed      bool
}

type monitor struct {
	x, y, w, h int
	name       string
}

func main() {
	log.SetFlags(0)

	user32 := syscall.NewLazyDLL("User32.dll")
	setProcessDpiAwarenessContext := user32.NewProc("SetProcessDpiAwarenessContext")
	r, _, err := setProcessDpiAwarenessContext.Call(uintptr(unsafe.Pointer(C.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)))
	if r == 0 {
		log.Printf("Warning: SetProcessDpiAwarenessContext() failed; detected monitor resolutions might be incorrect if DPI scaling is being used: %v", err)
	}
	settings := parseFlags(os.Args[1:])

	monitors := getMonitors()
	xMin := int(int32(0x7fffffff))
	xMax := int(int32(-0x7fffffff - 1))
	yMin := int(int32(0x7fffffff))
	yMax := int(int32(-0x7fffffff - 1))
	for _, mon := range monitors {
		xMin = min(xMin, mon.x)
		xMax = max(xMax, mon.x+mon.w)
		yMin = min(yMin, mon.y)
		yMax = max(yMax, mon.y+mon.h)
	}
	if len(settings.inFilePaths) > len(monitors) {
		log.Fatalf("Specified %d input wallpaper file paths, but only %d monitors were found.", len(settings.inFilePaths), len(monitors))
	}

	imgOut := image.NewNRGBA(image.Rectangle{Min: image.Pt(xMin, yMin), Max: image.Pt(xMax, yMax)})
	for i, inFilePath := range settings.inFilePaths {
		fileIn, err := os.Open(inFilePath)
		if err != nil {
			log.Fatalf("Fatal: could not open file %#q for reading: %v", inFilePath, err)
		}
		imgIn, _, err := image.Decode(fileIn)
		if err != nil {
			log.Fatalf("Fatal: could not decode image from file %#q: %v", inFilePath, err)
		}
		err = fileIn.Close()
		if err != nil {
			log.Printf("Warning: error closing handle to file %#q: %v", inFilePath, err)
		}
		boundsIn := imgIn.Bounds()
		xIn := boundsIn.Min.X
		yIn := boundsIn.Min.Y
		wIn := boundsIn.Max.X - boundsIn.Min.X
		hIn := boundsIn.Max.Y - boundsIn.Min.Y
		mon := monitors[i]
		if mon.w != wIn || mon.h != hIn {
			log.Fatalf("Fatal: image %d (%#q) dimensions %dx%d do no match Monitor %d (%#q) dimensions %dx%d.", i, inFilePath, wIn, hIn, i, mon.name, mon.w, mon.h)
		}
		for y := 0; y < hIn; y++ {
			for x := 0; x < wIn; x++ {
				imgOut.Set(mon.x+x, mon.y+y, imgIn.At(xIn+x, yIn+y))
			}
		}
	}

	fileOut, err := os.OpenFile(settings.outFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("Fatal: could not open file %#q for writing: %v.", settings.outFilePath, err)
	}

	format := ""
	if strings.HasSuffix(strings.ToLower(settings.outFilePath), ".png") {
		format = "PNG"
	} else if strings.HasSuffix(strings.ToLower(settings.outFilePath), ".jpg") || strings.HasSuffix(strings.ToLower(settings.outFilePath), ".jpeg") {
		format = "JPEG"
	}

	if format == "" {
		log.Print("Warning: could not determine supported image format from file extension of outFilePath. Defaulting to PNG.")
		log.Print("Supported extensions are .png (PNG), .jpeg (JPEG) and .jpg (JPEG).")
	}
	if format == "" || format == "PNG" {
		err = png.Encode(fileOut, imgOut)
	} else if format == "JPEG" {
		options := jpeg.Options{Quality: 100}
		err = jpeg.Encode(fileOut, imgOut, &options)
	} else {
		panic(fmt.Sprintf("detected but unimplemented format %v", format))
	}

	if err != nil {
		log.Fatalf("Fatal: could not encode image in file %#q: %v.", settings.outFilePath, err)
	}
	err = fileOut.Close()
	if err != nil {
		log.Printf("Warning: error closing handle to file %#q: %v", settings.outFilePath, err)
	}
}

func getMonitors() (monitors []monitor) {
	var cMonsData cMonitorEnumData

	arg0 := (C.HDC)(unsafe.Pointer(uintptr(0)))
	arg1 := (*C.RECT)(unsafe.Pointer(uintptr(0)))
	arg2 := (*[0]byte)(unsafe.Pointer(uintptr(C.monitorEnumProc)))
	arg3 := C.longlong(uintptr(unsafe.Pointer(&cMonsData)))
	r1 := C.EnumDisplayMonitors(arg0, arg1, arg2, arg3)

	if r1 == 0 || cMonsData.failed {
		if !cMonsData.failed {
			log.Fatalf("EnumDisplayMonitors() failed with WINAPI error code %d.", C.GetLastError())
		} else {
			log.Fatalf("GetMonitorInfoW() failed with WINAPI error code %d.", cMonsData.failCode)
		}
	}

	for i := 0; i < int(cMonsData.numMonitors); i++ {
		cMon := cMonsData.monitors[i]
		var mon monitor
		mon.x = int(cMon.x)
		mon.y = int(cMon.y)
		mon.w = int(cMon.w)
		mon.h = int(cMon.h)
		mon.name = stringFromWChars(unsafe.Pointer(&cMon.name[0]))
		monitors = append(monitors, mon)
	}
	return monitors
}

func stringFromWChars(wcharp unsafe.Pointer) string {
	buf := make([]uint16, 0, 32)
	var i uintptr = 0
	for {
		b1 := *((*byte)(unsafe.Pointer(uintptr(wcharp) + i)))
		b2 := *((*byte)(unsafe.Pointer(uintptr(wcharp) + i + 1)))
		if b1 == 0 && b2 == 0 {
			break
		}
		buf = append(buf, uint16(b1)|(uint16(b2)<<8))
		i += 2
	}
	return string(utf16.Decode(buf))
}

type flags struct {
	inFilePaths []string
	outFilePath string
}

func parseFlags(args []string) *flags {
	var f flags
	fs := flag.NewFlagSet("default", flag.ContinueOnError)
	var inFilePaths [maxMonitors]string
	for i := 0; i < len(inFilePaths); i++ {
		fs.StringVar(&inFilePaths[i], "i"+strconv.Itoa(i), "", fmt.Sprintf("Monitor %d wallpaper file path", i))
	}
	fs.StringVar(&f.outFilePath, "o", "", "Output file path")
	err := fs.Parse(args)
	if err != nil {
		printUsageAndFail(err)
	}
	for i := 0; i < len(inFilePaths); i++ {
		if inFilePaths[i] != "" {
			f.inFilePaths = append(f.inFilePaths, inFilePaths[i])
		}
	}
	if len(f.inFilePaths) == 0 || f.outFilePath == "" {
		printUsageAndFail(err)
	}
	return &f
}

func printUsageAndFail(err error) {
	if err != nil {
		log.Printf("Incorrect usage: %v.", err)
	}
	log.Print("Usage: wallpapercompiler -i0 inFile0Path [-i1 inFile1Path [...]] -o outFilePath")
	log.Print("List of monitors:")
	printMonitors(getMonitors())
	os.Exit(1)
}

func printMonitors(monitors []monitor) {
	for i, mon := range monitors {
		log.Printf("  Monitor %d (%#q) at (%d, %d) has dimensions %dx%d.", i, mon.name, mon.x, mon.y, mon.w, mon.h)
	}
}
