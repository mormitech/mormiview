# mormiView Help

mormiView is a picture viewer that gets out of your way, with a small Paint-like editor and a chat with Claude built in. This page is the manual.
In the app, **Help → mormiView Help** opens it. Below, **Ctrl** is **Cmd** on a Mac.

- [Quick start](#quick-start)
- [Looking at pictures](#looking-at-pictures)
- [Editing](#editing)
- [Selecting: rectangle and free-hand](#selecting)
- [Chat with Claude](#chat-with-claude)
- [Saving, closing, deleting](#saving-closing-deleting)
- [All the keys](#all-the-keys)
- [When something does not work](#when-something-does-not-work)

---

## Quick start

1. **Open a picture.** Press **O**, or use *Open with…* in your file manager (see the [README](../README.md#open-with)).
2. **Move around.** Arrow keys go to the next and the previous picture of the folder. The wheel zooms, dragging moves the picture.
3. **Press E** to edit. The tool rail appears on the left. Draw, type, select, paste.
4. **Press C** to chat with Claude about the picture, and ask it to change it.
5. **Save with S.** Nothing is written to the file until you do.

A window with no picture offers big buttons: **Open file…** (**O**), **Open from clipboard** (**Ctrl+V**), **Create image** (**N**) and **Scan** (a picture from a scanner opens as a new, unsaved picture; on macOS it opens Image Capture and takes the first new picture that turns up in Pictures, Desktop, Documents or Downloads). **Reopen last closed** brings back a closed picture with its changes and undo history. Under the buttons are the five pictures opened last; *File → Open Recent* has the last ten. Undo never goes back to the home screen.

---

## Looking at pictures

mormiView shows JPEG, PNG, GIF, BMP, WebP, ICO, SVG, AVIF and TIFF.

### Zoom

The keys **1** to **6** set how the picture sits in the window, as in IrfanView. The mode stays when you go to another picture.

| Key | Mode |
|---|---|
| **1** | Original size, 100%, however small or big |
| **2** | Fit the window with a margin (small pictures stay 100%). This is how every new window starts |
| **3** | Fit every picture to the window borders, keeping its proportions |
| **4** | Stretch every picture to the window borders |
| **5** | Zoom until the side edges touch the window |
| **6** | Zoom until the top and bottom edges touch the window |

**+** and **-** zoom in steps, the **mouse wheel** zooms around the pointer, and **dragging** moves the picture. On a Mac, two fingers on the trackpad move the picture and a pinch zooms. A double click does not change the zoom.

### Moving through a folder

**Arrow keys** (right: next) and **Backspace** (previous) go through the pictures of the folder in alphabetical order, round and round. The title of the window says the size, the place in the folder (`3 / 5`) and the zoom.

### Turning, full screen

- **R** turns the view a quarter clockwise, **Shift+R** counterclockwise. Only the view turns; the file stays as it is until you save. With a part selected, **R** turns only that part instead (see Image and Effects).
- **F** or **Enter** is full screen. There is no title or bar then; the bar comes up when the pointer is at the bottom edge, and for two seconds when another picture arrives (opened, pasted or stepped to). **Esc** does not leave full screen (two Escs close the picture).

### Animated GIFs

A GIF with several frames plays, and a small player appears at the bottom: **pause/play**, **a frame back**, **a frame forward** and the frame counter (for example `14 / 22`). Outside edit mode the player fades away while the animation runs, and comes back when you pause or step. **Space** plays and pauses; **Shift+wheel**, **Up** and **Down** stop the animation and step through the frames, round and round. Starting the animation lets a selection go. In edit mode, pause on a frame and every tool draws on that frame only; see Image and Effects for making and saving animations.

### Looking closely

In edit mode the picture is shown pixelated, every pixel a sharp square, and the bar shows the pixel under the pointer: x and y, R G B and the hex color. Zoom to 800% to see single pixels.

A picture with transparent parts has chequered squares behind it.

---

## Editing

**E** (or **Tab**) turns edit mode on and off. The title says `[edit]`, the status capsule shows an **Edit** label, and the tool rail appears. Edit mode needs a picture; **File → Create Image** makes one.

### The tools

| Tool | What it does |
|---|---|
| **Select** | Drag to select a rectangle. Drag an edge or the inside of the selection to resize or move just the frame (the picture is not touched, in view mode and in edit mode alike). Hold **Cmd** (Mac) or **Ctrl** (Windows, Linux) and drag an edge to stretch the selected part, or its inside to move it; add **Shift** on a corner to keep the proportions (Shift alone on a corner does the same for the frame); the place it came from turns white (transparent where the picture can have it). A part that floats (a pasted picture, or one you lifted this way) moves and stretches with a plain drag until it is applied (a click beside it, or Esc). With nothing selected, the image has a grip in the middle of each edge and on each corner, as in Paint: drag one out to add room to the canvas on that side (in the background color, or transparent when the image has transparent parts), or in to cut the image there; a dashed frame shows the new size, and Cmd/Ctrl+Z takes it back |
| **Polygon select** | Select any shape, free-hand: see [below](#selecting) |
| **Pencil** | Drag to draw. Left button: first color. Right button: second color. The ribbon above the bottom bar sets the width |
| **Line**, **Rectangle**, **Circle and ellipse** | Drag them out. **Shift** makes a line at a multiple of 45°, a square, a true circle. The ribbon sets the width, solid / dashed / dotted, and outline, outline with fill, or fill |
| **Fill** (paint bucket) | Click fills the area of the same color with the first color, right click with the second. The ribbon has a tolerance: 0% fills exactly that color, more takes in colors that are close |
| **Text** | Click, type, click elsewhere to put it in (**Ctrl+Enter** also does, **Esc** drops it). Font, size, bold and italic are in the ribbon |
| **Pick** | Click takes the color under the pointer as the first color (right click: the second), then the tool you had comes back |

The two **color wells** at the bottom of the rail are the first and the second color. Click one for the palette, or *Edit colors…* for the system's picker.

**Ctrl+V** lays the picture from the clipboard over this one as a floating part you can drag and stretch. Delete, Backspace or Esc take it away; a click beside it lays it down.

### Undo, redo, discard

- **Ctrl+Z** undoes, **Ctrl+Shift+Z** (or **Ctrl+Y**) redoes. Every change of the picture can be undone. Zooming, moving and selecting are not steps.
- **File → Discard Changes**, or the round-arrow button at the bottom right of the edit bar, takes the picture back to what it was when you opened it (the file on the disk, or the pasted or new picture). It is a step too: **undo brings your changes back**, redo discards them again.
- A picture with changes has a **●** after its name.

### Image and Effects

The **Image** menu also makes animations: **Start Animation** (the picture becomes frame 1), **Add Frame** (a copy of the frame that is shown, after it), **Delete Frame** and **Export Frames as PNG…**; the frame player has the same Add and Delete buttons and a field for how many milliseconds each frame is shown. The chat can make an animation too: ask for it, and the frames are in the player when it is done. Save As then offers the whole animation as a GIF, or only the frame that is shown. The **Image** menu turns and flips the whole picture: **Rotate Right**, **Rotate Left** (a quarter turn, like **R**), **Rotate by Angle…** (any angle from -360° to 360°, smooth; the corners either make the picture bigger or are cut off) and **Flip Horizontal/Vertical**. A right click on the picture (with nothing selected) offers the same turn, an **Effects** submenu, and Copy, Select all, Save as… and Print…. With a part selected, the right-click menu also has Cut, Crop to selection and Clear, and **R**, Rotate Right/Left and Rotate by Angle… turn only the selected part: a quarter turn is stretched to fill the frame again, and any other angle is blown up just enough that no corner is empty. Turning the same part again and again starts from its pixels before the first turn, so it does not get blurrier, until you let the selection go. The same goes for the whole picture: turns that follow each other, with nothing else done to the picture in between, are all made from the picture before the first one (any other change, or an undo, starts afresh).

The **Effects** menu changes the picture, or only the part you selected (a rectangle or a free-hand polygon; with nothing selected, all of it). **Clear** (also **Delete** or **Backspace** while something is selected) fills the selected part with the second color, as Delete does in Paint; the selection stays, so you can undo and try another effect. The effects: **Blur**, **Pixelize**, **Emboss**, **Explosion** (a burst from the middle), **Censor** (a mosaic too coarse to read, for faces, plates and names), **Highlight** (the selected part stays as it is and gets a thin border, and the rest of the picture is pushed back so that the part stands out: **Light** only shades the rest a little, **Medium** makes it darker and paler with a soft shadow round the part but still readable, **Full** also blurs it), **Sharpen**, **Grayscale**, **Negative**, **Sepia**, **Brighter/Darker**, **More/Less Contrast**, **More/Less Color**, **Warmer/Cooler**, **Auto Levels**, and **Fill** with the foreground or background color. Each is one step of the history (Ctrl+Z takes it back); using one again makes it stronger. Claude can do the same, with settings, through the chat.

The **AI** menu has **Web search and fetch**, which says whether Claude may use the web in the chat (on by default).

### Create image

**N** (or **File → Create Image**) makes a new white 1280 × 720 picture and starts edit mode. **Hold Shift** and click the *Create image* button (or press **Shift+N**) to choose the **width, height and background** (white, transparent or a color) first. mormiView remembers your last choice.

A new image, and a picture pasted from the clipboard, count as unchanged until you do something to them: you can close them with **Esc Esc** and nothing asks.

---

## Selecting

There are two ways to select, both in the tool rail.

### Rectangle

With the **Select** tool, drag. The moment you let go, the selection is **on the clipboard**, and its size is written in the middle. **Ctrl+C** copies the whole picture when nothing is selected.

### Free-hand (polygon)

With the **Polygon select** tool (the second one), **draw round the part you want**, any shape, as you would with a pencil. The dashed line follows the mouse. When you let go, mormiView joins the end to the start and turns your line into a **polygon** with a few corners, within about a pixel of what you drew.

You can also **hold the right mouse button** and draw, with the Select tool or the Polygon select tool, without changing tools.

Once it is set:

| Key | |
|---|---|
| **Delete** (or Backspace) | Fills the polygon with the second color |
| **Ctrl+C** | Copies what is inside it (the rest is transparent) |
| **Esc** | Lets it go |

**Esc**, or a **right click** with the left button down, drops a selection while you are still drawing it (the rectangle too).

The polygon is also what Claude works on when you write *"remove the selected part"*.

---

## Chat with Claude

Press **C**, or click the speech bubble at the bottom right of the picture, to open the chat: a floating window grows out of it, almost as tall as the window, and it shrinks back into the bubble when you minimize it. Drag its left edge to change the width. Ask for what you want in your own words; Claude looks at the picture and changes it with mormiView's tools. Every change is a step you can undo.

### What you need

- **Claude Code**, installed once ([claude.com/product/claude-code](https://claude.com/product/claude-code)).
- A **Claude account**. If you are not signed in, the panel shows **Sign in with Claude**; it opens your browser. mormiView never sees your password or the sign-in.

The chat uses **your Claude subscription**. No API key is needed, and no terminal has to be open.

### What to ask

- *"What is in this picture?"*
- *"Draw a red circle round the car."*
- *"Remove the label from the T-shirt."* / *"Remove all the post-it notes' writing."*
- *"Remove the selected part"*, after you drew a polygon round something.
- *"Make the canvas 200 pixels wider on the right."* / *"Crop to the left half."*
- *"Create a 32 × 32 pixel art sprite of a spaceship."*
- *"Start again"* (discards the changes; undo brings them back).

To point at something, **select it** (rectangle or free-hand) and say *"this"* or *"the selected part"*. Or **mark it** with a rectangle or an arrow and say *"remove what I marked"*; Claude takes the marking away first and keeps the area in mind.

### The panel

- Answers stream in, formatted. What Claude does with the tools is one line (*Processing… · 14 actions · 1 failed*); click it for the list.
- **Stop** ends an answer. **New** starts a new conversation. The **bar button** minimizes the panel; drag its **left border** to change its width. **Enter** sends, **Shift+Enter** adds a line.

### Taking things away

Claude is good at taking small things out of a photo: letters, labels, spots, scratches, wires. It works piece by piece, like a retoucher: it outlines the thing exactly, fills it from the surroundings, looks at the result and fixes what is left. It copies pictures of the surroundings, so it is best on backgrounds like paper, walls, sky, water, grass or fabric. **It does not invent new objects**; for those, results vary.

### Your pictures and your privacy

mormiView has no accounts, ads or telemetry. The chat is the one place where a picture leaves the computer, and only when **you** ask: Claude looks at the picture (screenshots of it) through your own Claude Code, so **what you chat about is sent to Anthropic**, under your Claude account's terms. Nothing is sent when the panel is closed.

Claude can use mormiView's picture tools and web search and fetch, and nothing else: no files, no shell. The **AI** menu turns the web off. It reaches the picture through a private connection inside the window that no other program can use.

---

## Saving, closing, deleting

- **S** (Ctrl+S, *File → Save As…*) saves PNG, JPEG, GIF, BMP or TIFF. For a picture from a file with changes it first asks **Overwrite**, **Save As** (the dialog offers the name with the date and time added) or **Cancel**. With a selection, **S** saves just the selection. A picture with transparent parts is offered as PNG.
- **Closing** a picture with changes asks **Save**, **Discard** or **Cancel**: **Ctrl+W**, **Esc Esc**, the **✕**, going to another picture, or closing the window. **File → Close Image** (Ctrl+Shift+W) closes just the picture.
- **D** (*File → Delete Image…*) asks, then deletes the file and shows the next picture. On a Mac the file goes to the Trash; on Windows and Linux it is deleted for good. The menu item is greyed out for a picture that has no file.

---

## All the keys

| Key | |
|---|---|
| **O** / **Ctrl+O** | Open a picture |
| **N** / **Shift+N** | Create an image / choose its size first |
| **Ctrl+N** | New window |
| **E**, **Tab** | Edit mode on and off |
| **C** | The chat panel |
| **1** – **6** | Zoom modes |
| **+**, **-**, wheel | Zoom |
| **Arrow keys**, **Backspace** | Next, previous picture |
| **Space** | Play or pause an animated GIF |
| **Up**, **Down**, **Shift+wheel** | An animated GIF: a frame back, a frame forward (round and round; the animation stops) |
| **R**, **Shift+R** | Turn the view |
| **F**, **Enter** | Full screen |
| **S** | Save As |
| **D** | Delete the file |
| **Ctrl+C** | Copy the selection (or the whole picture) |
| **Ctrl+V** | Paste: a small picture floats over the picture as a part you can move; one about as big as the picture asks whether to make it a new image |
| **Ctrl+Z**, **Ctrl+Shift+Z** | Undo, redo |
| **Delete** | Fill a polygon selection with the second color; take away a floating part |
| **Esc** | With a drawing tool: back to the rectangle select tool; drop a selection or a drawing; let go of a selection; **Esc Esc** closes the picture |
| **Ctrl+W**, **Ctrl+Shift+W** | Close the picture or the window; close just the picture |
| **F1** | This help (Windows and Linux) |

---

## When something does not work

- **The chat says Claude Code is not installed.** Install it, then press *Check again*. mormiView looks in the usual places and in your login shell's path.
- **The sign-in does not finish.** Finish it in the browser tab; if no tab opened, the address is shown in the panel. *Cancel* starts again.
- **The chat says it is signed in with an API key** (the account line is red). Chats are then billed by use. Sign in with *Claude* to use your subscription.
- **Copying and pasting pictures does nothing on Linux.** Install `xclip` (X11) or `wl-clipboard` (Wayland).
- **macOS says the app is from an unidentified developer.** mormiView is not code-signed yet: right-click it → **Open** the first time.
- **A picture does not open.** mormiView opens the types listed above; AVIF and WebP need a web view that has them.
- **Something else.** Report it at [github.com/mormitech/mormiview/issues](https://github.com/mormitech/mormiview/issues) with the version number from the title bar.

mormiView is free and open source (MIT). Made by [Mormitech](https://mormitech.com).
