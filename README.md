# wallpapercompiler

On Windows, you cannot set a wallpaper per monitor. You have to create one large image which spans the entire desktop that contains each wallpaper at the correct location. This tool can construct that image, given you have the wallpapers you want to use in the exact resolutions of their target monitors.

## Example Usage (Command Line)

```cmd
> wallpapercompiler
Usage: wallpapercompiler -i0 inFile0Path [-i1 inFile1Path [...]] -o outFilePath
List of monitors:
  Monitor 0 (`\\.\DISPLAY1`) at (2560, 360) has dimensions 1920x1080.
  Monitor 1 (`\\.\DISPLAY2`) at (0, 0) has dimensions 2560x1440.
  Monitor 2 (`\\.\DISPLAY3`) at (-3840, 0) has dimensions 3840x2160.
```

The above invocation causes all connected monitors to be listed. You should be able to tell by coordinates and resolution which is which and remember the ID.

```cmd
> wallpapercompiler -i0 "D:\cat_full_hd.png" -i1 "D:\dog_qhd.png" -i2 "D:\mouse_4k.jpeg" -o "D:\wallpaper.png"
```

The above command will write the image file `D:\wallpaper.png` with the provided images placed at the according monitor's location within the image. (`-i0` specifies image for monitor 0. `-i1` specifies image for monitor 1. etc.)
