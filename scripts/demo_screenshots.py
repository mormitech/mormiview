#!/usr/bin/env python3
"""Makes the screenshots of the README and the manual with mormiView in demo mode (macOS; docs/control.md).

    scripts/demo_screenshots.py SAMPLES_DIR [OUT_DIR] [APP] [hero|palette]   (hero: the hero and the AI provider dialog)

SAMPLES_DIR holds berries.jpg, city.jpg, coast.jpg, lake.jpg, frozen-lake.jpg (free pictures, no people in them) and bounce.gif (an animation of
our own); keep it outside your home folder: the app is started with --demo, which refuses pictures from there and shows no recent pictures, account,
e-mail or window state of yours. OUT_DIR defaults to docs/screenshots (WebP when Pillow is installed, else PNG), APP to build/bin/mormiview.app.
The window is a fixed size, so the pictures look alike from one run to the next. What the system draws (its dialogs and the menu bar) and the chat
(it needs a signed-in Claude Code) are not made here.
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mormiview_ctl import MormiView  # noqa: E402


def main(samples, out="docs/screenshots", app="build/bin/mormiview.app", only=""):
    mc = MormiView.start(app, os.path.join(samples, "coast.jpg"))

    def shot(name):
        png = os.path.join(out, name + ".png")
        mc.shot(png)
        try:  # WebP keeps the transparent corners and the shadow, at a tenth of the size (Pillow); without it the PNG stays
            from PIL import Image
            Image.open(png).save(png[:-4] + ".webp", "WEBP", quality=90, method=6)
            os.remove(png)
        except ImportError:
            pass
        print("  ", name)

    def px(fx, fy):  # a place of the picture, as fractions of its size
        w, h = mc.state()["size"]
        return [round(fx * w), round(fy * h)]

    def open_picture(name, mode=2):
        mc.cmd("open", path=os.path.join(samples, name))
        mc.cmd("mode", mode=mode)

    def palette_shot():
        open_picture("coast.jpg")
        mc.cmd("edit")
        mc.cmd("color", fg="#FFF200")
        mc.cmd("tool", name="ellipse")
        mc.cmd("option", width=8, style="solid", fill="outline")
        a, b = px(0.635, 0.52), px(0.715, 0.625)  # one ring round the people on the beach
        mc.cmd("shape", kind="ellipse", x0=a[0], y0=a[1], x1=b[0], y1=b[1], color="#FFF200", width=8, style="solid", fill="outline")
        mc.cmd("tool", name="select")
        mc.cmd("palette", open="fg")
        shot("06-palette")
        mc.cmd("palette")

    mc.cmd("window", w=1500, h=960)
    mc.cmd("mode", mode=2)
    shot("01-view")

    # the hero: edit mode, a selection on the sky and the chat open with a friendly request (an unsent draft, nothing drawn)
    mc.cmd("edit")
    a, b = px(0.02, 0.03), px(0.62, 0.45)  # the evening sky
    mc.cmd("select", x0=a[0], y0=a[1], x1=b[0], y1=b[1])
    mc.cmd("chat", text="Warm up the evening sky a little")
    shot("00-hero")
    # the chat with an API key: the provider dialog, about to change the model (a made-up key; Cancel keeps nothing)
    mc.cmd("aidlg", provider="openrouter", key="sk-or-v1-demo-not-a-real-key-0000000000", model="anthropic/claude-sonnet-5.5")
    shot("22-ai-provider")
    mc.cmd("aidlg", close=True)
    mc.cmd("chat", on=False, wait=600)
    mc.cmd("polygon", pts=[])
    mc.cmd("edit", on=False)
    if only == "hero":
        mc.quit()
        return
    if only == "palette":
        palette_shot()
        mc.quit()
        return

    # the home screen, with the pictures opened so far under Recent
    for name in ("frozen-lake.jpg", "lake.jpg", "city.jpg", "berries.jpg", "coast.jpg"):
        open_picture(name)
    mc.cmd("menu", name="close-image")
    shot("02-home")

    # full screen: the bar shows for two seconds when another picture arrives
    open_picture("city.jpg")
    mc.cmd("fullscreen", on=True)
    open_picture("berries.jpg")
    shot("03-fullscreen-bar")
    mc.cmd("fullscreen", on=False)

    # zoom: look at the details, and draw exactly
    open_picture("city.jpg")
    mc.cmd("edit")
    lamp = px(0.69, 0.69)
    mc.cmd("tool", name="select")
    mc.cmd("zoom", scale=8, x=lamp[0], y=lamp[1])
    mc.cmd("pixel", x=lamp[0] + 5, y=lamp[1] + 3)
    shot("04-zoom-pixels")

    # draw with the tools, in the coordinates of the picture
    open_picture("coast.jpg")
    mc.cmd("edit")
    mc.cmd("color", fg="#ED1C24")
    mc.cmd("tool", name="pen")
    mc.cmd("option", width=14)
    ring = [px(0.78, 0.36), px(0.79, 0.31), px(0.825, 0.295), px(0.862, 0.315), px(0.866, 0.37), px(0.83, 0.405), px(0.79, 0.385), px(0.78, 0.35)]  # round the palm trees on the cliff
    mc.cmd("shape", kind="path", pts=ring, color="#ED1C24", width=14)
    mc.cmd("color", fg="#FFF200")
    mc.cmd("tool", name="ellipse")
    mc.cmd("option", width=10, style="solid", fill="outline")
    a, b = px(0.60, 0.53), px(0.72, 0.66)
    mc.cmd("shape", kind="ellipse", x0=a[0], y0=a[1], x1=b[0], y1=b[1], color="#FFF200", width=10, style="solid", fill="outline")
    mc.cmd("color", fg="#FFFFFF")
    mc.cmd("tool", name="rect")
    mc.cmd("option", width=8, style="dashed", fill="outline")
    a, b = px(0.30, 0.10), px(0.58, 0.30)
    mc.cmd("shape", kind="rect", x0=a[0], y0=a[1], x1=b[0], y1=b[1], color="#FFFFFF", width=8, style="dashed", fill="outline")
    mc.cmd("tool", name="line")
    mc.cmd("option", width=10, style="solid")
    a, b = px(0.24, 0.50), px(0.40, 0.44)
    mc.cmd("shape", kind="line", x0=a[0], y0=a[1], x1=b[0], y1=b[1], color="#FFFFFF", width=10, style="solid")
    mc.cmd("color", fg="#FFF200")
    mc.cmd("option", font="system-ui, sans-serif", size=190, bold=True)
    mc.cmd("text", x=px(0.06, 0)[0], y=px(0, 0.80)[1], value="Evening walk", color="#FFF200")
    mc.cmd("tool", name="rect")  # the ribbon of a shape tool stays on the screen
    shot("05-edit-draw")

    # the colors: a clean picture with one quiet mark on it
    palette_shot()

    # effects on a part: a mosaic over a sign, and half of a picture in gray
    mc.answer("discard", "discard")
    open_picture("city.jpg")
    mc.cmd("tool", name="select")
    a, b = px(0.87, 0.39), px(0.97, 0.52)  # the slanted neon sign on the right
    mc.cmd("select", x0=a[0], y0=a[1], x1=b[0], y1=b[1])
    mc.cmd("effect", op="censor", selection=True, recent=True)
    shot("10-effect-censor")
    mc.answer("discard", "discard")
    open_picture("berries.jpg")
    mc.cmd("tool", name="select")
    a, b = px(0.0, 0.0), px(0.5, 1.0)
    mc.cmd("select", x0=a[0], y0=a[1], x1=b[0], y1=b[1])
    mc.cmd("effect", op="grayscale", selection=True, recent=True)
    mc.cmd("effect", op="sharpen", selection=True, recent=True)
    shot("11-effect-part")

    # Highlight: the people on the beach stay, the rest of the picture is pushed back (medium: dimmed and paler, still readable)
    mc.answer("discard", "discard")
    open_picture("coast.jpg")
    mc.cmd("tool", name="select")
    a, b = px(0.60, 0.50), px(0.74, 0.66)
    mc.cmd("select", x0=a[0], y0=a[1], x1=b[0], y1=b[1])
    mc.cmd("effect", op="highlight", level="medium", selection=True)
    shot("21-effect-highlight")

    # a free-hand selection, and the right-click menus
    mc.answer("discard", "discard")
    open_picture("berries.jpg")
    mc.cmd("tool", name="select")
    mc.cmd("polygon", pts=[px(0.33, 0.20), px(0.60, 0.14), px(0.74, 0.44), px(0.52, 0.64), px(0.30, 0.46)])
    shot("07-polygon-selection")
    mc.cmd("ctxmenu", sub="Effects")
    shot("08-menu-selection")
    mc.cmd("closemenu")
    mc.cmd("polygon", pts=[])
    mc.cmd("ctxmenu")
    shot("09-menu-picture")
    mc.cmd("closemenu")

    # turning a part, and fine rotation of the whole picture
    a, b = px(0.30, 0.25), px(0.70, 0.75)
    mc.cmd("select", x0=a[0], y0=a[1], x1=b[0], y1=b[1])
    mc.cmd("effect", op="rotate", angle=18, selection=True)
    shot("13-rotate-selection")
    mc.answer("discard", "discard")
    open_picture("lake.jpg")
    mc.cmd("rotdlg", angle=6)
    shot("12-rotate-dialog")
    mc.cmd("rotdlg", close=True)

    # an animated GIF: the player, and drawing on one frame
    mc.answer("discard", "discard")
    mc.cmd("edit", on=False)
    open_picture("bounce.gif")
    mc.cmd("gifplay", on=False)
    for _ in range(5):
        mc.cmd("gifplay", step=1)
    shot("14-gif-player")
    mc.cmd("edit")
    mc.cmd("color", fg="#ED1C24")
    mc.cmd("tool", name="pen")
    mc.cmd("option", width=6)
    mc.cmd("shape", kind="path", pts=[px(0.2, 0.2), px(0.35, 0.12), px(0.5, 0.2), px(0.65, 0.12), px(0.8, 0.2)], color="#ED1C24", width=6)
    shot("15-gif-frame-edit")

    # a tooltip
    mc.answer("discard", "discard")
    open_picture("berries.jpg")
    mc.cmd("tool", name="select")
    mc.cmd("tip", id="tool-poly")
    shot("20-tooltip")

    mc.quit()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(2)
    main(*sys.argv[1:5])
