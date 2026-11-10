# Changelog

All notable changes to mcView are listed here, newest first. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the versions are the [releases](https://github.com/mormitech/mcview/releases).

## 0.57.8 – 2026-11-10

### Changed

- File → Close is greyed out while no picture is open; Cmd+W (Ctrl+W) still closes the empty window.

## 0.57.7 – 2026-11-10

### Changed

- The Effects menu, too, shows only while a picture is open: without one the menu bar has File, AI and Help.
- macOS: the Help menu no longer has the system's Search field.

## 0.57.6 – 2026-11-10

### Changed

- The Edit and Image menus show only while a picture is open (desktop and web); without one the menu bar has File, Effects, AI and Help. Ctrl+V (Cmd+V) still pastes a picture from the clipboard.

## 0.57.5 – 2026-11-10

### Changed

- Web version: Help → About mormiView opens a dialog over the page instead of a new tab (Esc, × or a click beside it closes it); its text names the AI providers the chat can use.

## 0.57.4 – 2026-11-08

### Fixed

- Linux: Paste takes the picture in whatever type the clipboard offers it (PNG first, then BMP, TIFF, WebP, JPEG, GIF), not only PNG. Over a remote desktop (xrdp), where the clipboard has only image/bmp, a picture copied on the other computer now pastes.

## 0.57.3 – 2026-11-05

### Fixed

- Web version: while it starts, only the loading screen shows (in the theme's colors); the half-styled page no longer shows through before the program and its style have loaded.

## 0.57.2 – 2026-11-05

### Added

- Control mode: `aidlg` leaves the AI provider dialog open with a provider, a key and a model in it (for the screenshot of choosing a model with an API key); `close` cancels it.

## 0.57.1 – 2026-11-05

### Added

- Each release also has the web version as a zip, for hosting it: `mormiview-web-<version>.zip` with its SHA-256 (`.sha256`), and the same zip as `mormiview-web.zip`. The zip is the site folder (`index.html`, `assets/`, `mormiview.wasm`), with relative paths only, so it runs from any folder of a site; mormitech.com hosts it at mormitech.com/mormitools/mormiview/app/.

## 0.57.0 – 2026-11-05

### Added

- **The chat with an API key in the desktop app too**: AI → AI Provider and Key… chooses who answers in the chat panel: Claude Code with your Claude subscription (as before, the default), the Claude API with your API key, or OpenRouter with your OpenRouter key, each with its model. With an API key the panel talks to that API itself, with the same picture tools; the keys are kept in the settings folder in a file only you can read (`keys.json`), or only until the window is closed. When Claude Code is not installed or not signed in, the panel offers **Use an API key…**.
- A pencil next to the model at the top of the chat panel changes the provider and the model (in the app and in the web version).

### Fixed

- The model at the top of the chat panel was red when an API key was used in the web version: red is for Claude Code signed in with an API key instead of a subscription (billed by usage without your choosing it).
- A model that writes blank lines before a tool call no longer leaves an empty bubble in the chat.

## 0.56.0 – 2026-11-05

### Added

- **OpenRouter in the web version's chat**: AI → AI Provider and Key… chooses the provider, the Claude API or OpenRouter, each with its own API key and model. With OpenRouter the chat goes through openrouter.ai with your OpenRouter key and credits, to any of its models that can use tools and see pictures (Claude Opus 5.5 by default; the list comes from OpenRouter), with the same picture tools. Changing the provider starts a new conversation.

## 0.55.0 – 2026-11-04

### Added

- **mormiView in the browser**: a static web version that needs no install and no server (`scripts/build_web.sh` builds it into `frontend/dist-web`, for any static host). It is the same program: the Go code is built for WebAssembly and the page is the desktop app's, so viewing, the zoom modes, edit mode and all its tools, effects, selections, the clipboard, undo, animated GIFs, rotating, resizing and Save As work as they do in the app. See [docs/web.md](docs/web.md).
  - Pictures are opened with File → Open, File → Open Folder… (all the pictures of a folder, for the arrow keys) or by dropping files or a folder on the window. They are kept in the browser (IndexedDB), with the recent pictures and the settings, so they are still there after a reload and in a new tab.
  - What is saved (Save As, overwriting, a selection, exported frames) is also downloaded. The questions and the Save dialog are drawn by the page; the menu bar is the app's own menu, drawn at the top of the page, with its shortcuts.
  - The chat with Claude talks to the Claude API from the page with your own API key (AI → AI Provider and Key…, kept in the browser only), with the same picture tools as the app, and web search when AI → Web search and fetch is on.
  - Not in the browser: scanning, and the control mode for other programs. Printing uses the browser's print dialog.

### Changed

- The questions about unsaved changes, overwriting and saving an animation have the three buttons of the macOS version in the web version too (on Linux they are still two questions with Yes and No).

## 0.54.0 – 2026-10-29

### Added

- **Effects → Highlight**, with three levels (also in the right-click menu): the selected part (a box or a polygon) stays as it is and gets a thin light border with a faint dark edge, and the rest of the picture is pushed back. **Light** only shades the rest a little, **Medium** makes it darker and paler with a soft shadow round the part but keeps it readable, **Full** also blurs it. One step of the history, like the other effects. Claude can use it too (`apply_effect` highlight, with `level`, the strength and the border color).
- While a selection is drawn, moved or resized, the pixel coordinates of its top left and bottom right corners show as `[x, y]` above its top left and below its bottom right corner, next to the size in the middle. Zoomed in, a label whose place is out of the window goes inside the part of the selection that is seen (the size: in its middle).

### Changed

- The status bar is a floating capsule over the bottom of the picture instead of a bar across the window, with icons: the folder (its last part), an **Edit** label and an accent rim in edit mode, the pixel under the pointer (crosshair, color dot, hex code and the R, G, B numbers), and the selection as its two corners and its size. It stays clear of the tool rail and the chat button; an open dock (the palette, the options of a tool) sits at the bottom edge with the capsule above it, and the chat button no longer moves up when the bar shows.
- The capsule gets out of the way: when the pointer comes over it (also while dragging a selection), it fades over to the top of the window, and back as soon as the pointer leaves that place. Not in full screen outside edit mode, where the pointer at the bottom is what calls the bar up.
- A click inside a selection or on its edge that leaves it as it was (to the pixel) no longer copies it to the clipboard again when that very part is on the clipboard already. Ctrl+C (Cmd+C) and Copy in the menus still copy it every time.

### Fixed

- The previous and next picture keys do nothing on a pasted or a new picture (which is in no folder), instead of saying to save it first.
- A message in the status capsule shows in full and stands alone, instead of running out of the capsule.
- The recent pictures on the home screen are readable in full screen (light text on the black, in the light theme too), and their folder reads `~/Pictures` instead of `Pictures/~`.

## 0.51.12 – 2026-10-28

### Changed

- The title on the home screen and in the About box is the two-tone wordmark, "mormi" + "View", in the colors of mormiPrint and mormiDiskFree.

### Fixed

- Under `wails dev` every picture said "Cannot show": the Vite dev server answered the picture's request with the page itself. Vite now answers 404, so the request goes to the app. The built app was not affected.

## 0.51.11 – 2026-10-27

### Added

- macOS: the app holds a launcher, `Contents/Resources/bin/mormiview`, that the Homebrew cask links as the `mormiview` command, as other apps do (`code`, `subl`). It starts the app the way Finder does (so the system knows it), opens the pictures named after it, and gives the prompt back at once. Flags such as `--demo` are passed on.

## 0.51.10 – 2026-10-27

### Fixed

- macOS: started from a terminal by the `mormiview` command (the link that Homebrew installs), the Open dialog opened and collapsed at once, so no file could be opened. The system finds the app's identity from the path the program was started by, and through the link there was none. The program now starts itself again by the real path of the app, at once. (Started from Spotlight, Finder or `open`, it was fine.)

## 0.51.9 – 2026-10-27

### Added

- `scripts/make-cask.mjs`, `make-scoop-manifest.mjs` and `make-winget-manifest.mjs` write the Homebrew cask, the Scoop manifest and the winget manifests of a published release (checksums computed from the published files).

## 0.51.8 – 2026-10-27

### Added

- Control mode can open the chat panel with a draft in its box (`chat`) and show a pixel in the readout (`pixel` with `show`), for screenshots.

## 0.51.7 – 2026-10-27

### Added

- Control mode can open the right-click menu, the Rotate by angle dialog and a tooltip, and play, pause and step an animation, for screenshots of them (docs/control.md).

### Fixed

- The right-click menu of a free-hand (polygon) selection was the menu of the whole picture, and its Copy copied all of it. It is now the menu of the selection: Cut, Copy (what is inside the polygon), Effects, Rotate and Clear.

### Changed

- A demo run (`--demo`) shows nothing personal: it does not open pictures from your home folder, and the chat panel shows no account name, e-mail or plan.

## 0.51.5 – 2026-10-27

### Added

- `--demo` (also `--mock`): control mode on a free port, with a new empty settings folder, so that screenshots show no recent pictures or window state of the person making them. The demo scripts use it.

## 0.51.4 – 2026-10-27

### Added

- A **Clear** button next to Recent on the home screen empties the list of recent pictures (the files stay).

## 0.51.3 – 2026-10-27

### Changed

- The window title no longer carries the version number (it is still in the About window).

## 0.51.2 – 2026-10-27

### Changed

- Help → mormiView Help opens the manual at its new address, mormitech.com/mctools/mormiview/docs/ (the old addresses still redirect).

## 0.51.1 – 2026-10-27

### Changed

- The README and the manual describe the newer features: animated GIFs (player, frame editing, making and saving animations), Image and Effects menus, fine rotation, the right-click menu, Scan, recent pictures and Reopen last closed, tooltips.

## 0.51.0 – 2026-10-27

### Changed

- **mcView is now mormiView.** The app, the files of the release (`mormiview-<version>-…`), the repository (`mormitech/mormiview`), the command and the
  environment variables (`MORMIVIEW_CONTROL`, `MORMIVIEW_TRACE` …) carry the new name. Older entries of this list keep the name they were made under.
- The settings (recent pictures, window, AI settings) of mcView are moved to the new folder the first time mormiView starts.
- The Windows installer upgrades an mcView install and removes its entries; the Linux packages replace the `mcview` package.
- macOS: the app has a new identity, so Open With and the default app for picture types have to be chosen again, and the first start asks again to be opened.

## 0.50.3 – 2026-10-27

### Changed

- In full screen, the info bar at the bottom shows for two seconds when another picture arrives (opened, pasted, or stepped to), then hides again.

## 0.50.2 – 2026-10-27

### Fixed
- Saving an animation as the whole GIF offered a PNG name (`clipboard-….png`) and all picture types. The dialog now offers `animation-….gif` (or the file's name with .gif) and only the GIF type; a single frame is offered as `…-frame-N.png`.

## 0.50.1 – 2026-10-27

### Fixed
- An animation made from a new or pasted picture (and so one the chat made) did not show the frame player and the frame count: the page only looked for the player on pictures whose file name ends in .gif. It now shows it for every animation, at once.

### Changed
- The title of a picture that has no file says where it came from: [new image] for Create image, [scanned image] for a scan, [clipboard image] for a pasted one.
- The chat works on frames independently of the player: a tool called with a `frame` number changes that frame and the frame the user has on the screen stays (and a running animation runs on); `add_frame`, `duplicate_frame`, `delete_frame` and `move_frame` do not move the user's frame either (the player's own buttons do follow the change). `animation_info` tells which frame the user looks at.

## 0.50.0 – 2026-10-27

### Added
- Making animations. Image → Start Animation makes the picture frame 1 of an animation; Add Frame (a copy of the frame that is shown, after it), Delete Frame and Export Frames as PNG… (every frame as a numbered PNG file) are in the menu, and the frame player has Add and Delete buttons and a field for the milliseconds each frame is shown. Every change of the frames is a step of the history. Save As of the animation offers the whole GIF or the frame that is shown.
- The chat's Claude can make an animated GIF: new tools `animation_start`, `add_frame`, `duplicate_frame` (what to copy and where it goes), `delete_frame`, `move_frame`, `set_frame_delay`, `animation_info` and `frames_sheet` (all frames in one picture, to check the motion), and every tool that reads or changes pixels takes an optional `frame` number (the first is 1), so it can work on any frame without moving the user's player. Claude leaves the animation in the window; the user saves it.

## 0.49.5 – 2026-10-27

### Changed
- An animated GIF that runs outside edit mode fades its player away in under a second; pausing, stepping (Space, arrows, Shift+wheel) brings it back.

## 0.49.4 – 2026-10-27

### Added
- Animated GIF: Shift+wheel stops the animation and steps through the frames (wheel up a frame back, wheel down a frame forward), and the up and down arrow keys do the same. At either end it goes round to the other end, so it never stops. Outside edit mode the first step brings the player up.

## 0.49.3 – 2026-10-27

### Fixed
- Going to another animated GIF could show frames of the GIF before it: the player kept the pictures it had made for the old frames, and the new animation's frames could get the same keys. The player, the frames, the count and the timer of the old GIF are flushed at once when another picture is shown, and the keys are numbered per animation.

## 0.49.2 – 2026-10-27

### Changed
- Space no longer goes to the next picture (the right arrow does). With an animated GIF it plays and pauses the animation (outside edit mode, the first press brings the player up, paused).

## 0.49.1 – 2026-10-27

### Changed
- Starting the animation with Play lets the selection go (a selection belongs to one frame).

## 0.49.0 – 2026-10-27

### Added
- A GIF frame editor. An animated GIF in edit mode plays in the frame player (Pause/Play, one frame back, one frame forward, the frame counter). Paused or stepped on a frame, every tool draws on that frame, and the animation stays: the other frames are untouched, the player stays after the edit, and undo and redo bring the frame back. A GIF whose frames were changed also shows the player outside edit mode.
- Save As (also the S key) of an animation, stopped on a frame, asks whether to save the whole animation (a GIF with all frames and their delays) or only the frame that is shown (as PNG, by default named after the frame number). Saving to another type than GIF saves the frame that is shown.
- A change of the picture's size (resize, crop, canvas) ends the frame editing: the picture is a still then.

### Fixed
- The player no longer disappears after drawing on a frame.

## 0.48.2 – 2026-10-27

### Fixed
- Drawing on a paused frame of an animated GIF showed "Cannot show …" and an empty picture: the frame player put the GIF's own file back over the edited picture. It does that now only when edit mode is left.

## 0.48.1 – 2026-10-27

### Changed
- While an animated GIF plays and the tool rail is hidden, the bottom bar reaches the left edge of the window.

## 0.48.0 – 2026-10-27

### Added
- Recent pictures: the home screen lists the five pictures opened last (click to open), and File → Open Recent has the last ten, with Clear Menu. The list is kept for next time and shared by all windows.

## 0.47.0 – 2026-10-27

### Added
- A frame player for animated GIFs: in edit mode a small bar at the bottom shows Pause/Play, one frame back, one frame forward, and the frame counter (for example 14 / 22). Paused (or stepped) on a frame, that frame is what the pixel readout reads and what an edit starts from; an edit makes the picture a still, and the player goes away until you undo back to the GIF. While the animation plays the tool rail is hidden (there is nothing to draw on); pausing or stepping brings it back. Outside edit mode the GIF plays by itself as before.

## 0.46.0 – 2026-10-27

### Added
- The right-click menu shows a "Recent" group above Effects with the last three effects you used (the latest first; kept for next time), so a repeat is one click.

### Changed
- Esc, with a drawing tool other than the rectangular selection set (edit mode), returns to the rectangular selection tool.

## 0.45.1 – 2026-10-27

### Changed
- Undo no longer goes back to the home screen: opening, scanning or pasting a picture from the home screen is not undone, so undoing a lot stops at the picture (going to the next or previous picture of the folder is still undone). Reopen last closed still brings back a closed picture.

## 0.45.0 – 2026-10-27

### Added
- The right-click menu (of a selection, and of the whole picture) has an Effects submenu: an effect works on the selection, or on all of the picture when nothing is selected.
- A selected part can be turned: **R** and **Shift+R** (a quarter turn each way), Image → Rotate Right/Left, Rotate by Angle… (with any angle) and the right-click menu turn only the selected part (a box or a polygon) when there is a selection. A quarter turn is stretched to fill the frame again; any other angle is blown up just enough that the turned part reaches every edge. Turning the same part again and again starts from its pixels before the first turn, so it does not get blurrier (until the selection is let go).
- Consecutive fine rotations of the whole picture (nothing else changed it in between) are all made from the picture as it was before the first one, by the angles added up, so repeating them does not blur the picture or grow its canvas each time; any other change, or an undo, starts afresh.
- The chat's `apply_effect` has a `rotate` effect (a part, and angle).

## 0.44.1 – 2026-10-27

### Changed
- Esc minimizes the chat window while you are in it (the cursor in its text field, or anything inside it focused).

## 0.44.0 – 2026-10-27

### Added
- While Claude is working and the chat window is minimized, the chat button pulses with rings and its bubble bobs, so you can see that it is busy.

## 0.43.1 – 2026-10-27

### Fixed
- On macOS a small dropdown with system actions (Add To Contacts, Look Up, Translate…) could appear over text that the system reads in the picture (Live Text); the picture no longer takes the pointer, so it does not.

## 0.43.0 – 2026-10-27

### Added
- A new Image menu: Rotate Right, Rotate Left (a quarter turn, as the R key), Rotate by Angle…, Flip Horizontal and Flip Vertical.
- Fine rotation: Image → Rotate by Angle… turns the picture by any angle (smooth, -360° to 360°; with a slider), either making the picture bigger so that nothing is cut off or keeping its size and cutting the corners; the empty parts are transparent (or white where the file type has no transparency), white, or the second color. (The chat's `rotate_image` had it already.)
- A right click on the picture with no selection opens a menu of the whole picture: Copy, Select all, Save as…, Print… and Rotate by angle…. (Cut and Crop are only offered for a selection.)

## 0.42.1 – 2026-10-27

### Changed
- The cards of the home screen have their own subtle colors (amber, violet, blue, green, pink) on a soft tinted square behind the icon.

## 0.42.0 – 2026-10-27

### Added
- "Reopen last closed" on the home screen: after an image was closed, it brings the picture back with all its changes and its undo history (it is an undo of the close step, so Ctrl+Z does the same).

## 0.41.1 – 2026-10-27

### Changed
- A scanned picture counts as an unsaved change (the modified mark shows, and closing it asks), because a scan takes time and a scanner; the pictures from the file, the clipboard and Create image are still closed without a question.
- On macOS, Image Capture is quit when the scan has arrived (if mcView started it), and mcView gets the focus back.

## 0.41.0 – 2026-10-27

### Added
- A right click on a selection (a box or a free-hand polygon) opens a menu: Cut, Copy, Crop to selection, Save selection as…, Print… and Clear. Print sends the selected part to the default printer. A right drag still draws a new polygon. The chat's Claude can do the same with the new tools `cut_selection`, `crop_to_selection`, `save_selection` and `print_selection` (it already had `copy` and `clear_selection`).

## 0.40.3 – 2026-10-27

### Changed
- A click anywhere outside the chat window minimizes it.

## 0.40.2 – 2026-10-27

### Changed
- The chat window starts wider (460 px), and its New and Minimize buttons are colored rounded squares (blue and amber).

## 0.40.1 – 2026-10-27

### Changed
- Scan on macOS opens Image Capture and then opens the first new picture that turns up in Pictures, Desktop, Documents or Downloads (for up to ten minutes), so scan to one of those folders.

## 0.40.0 – 2026-10-27

### Changed
- The chat is a floating window, like a messenger's: it sits over the bottom right corner of the picture (which no longer narrows), reaches almost to the top, grows out of the chat button and shrinks back into it with an animation when minimized. The chat button moved to the bottom right, above the status bar when there is one.

## 0.39.0 – 2026-10-27

### Added
- Scan on the home screen: a picture from a scanner opens as a new, unsaved picture (Windows: the system scan dialog; Linux: SANE's `scanimage`; macOS opens Image Capture, which has no scan command to call).
- Tooltips: everything with a hint shows it after one second in a small dark bubble instead of the system tooltip.

### Changed
- The chat's New button is an empty-document icon and the minimize button has a nicer arrow-to-edge icon.

## 0.38.10 – 2026-10-27

Everything since 0.37.8, in one release: an Effects menu, an AI menu, many more tools for the chat's Claude, and a lot of polish.

### Added
- **Effects menu**: Clear, Blur, Pixelize, Emboss, Explosion, Censor, Sharpen, Grayscale, Negative, Sepia, Brighter/Darker, More/Less Contrast, More/Less Color, Warmer/Cooler, Auto Levels and Fill with the foreground color. Each works on the part that is selected (a polygon, or else the box), or on all of the picture when nothing is selected, and is one step of the history (using one again makes it stronger). The selection stays, also through undo and redo, so you can try one effect after the other.
- **Clear** (Effects → Clear, or **Delete** or **Backspace** while something is selected) fills the selected part with the second color, as Delete does in Paint. Without a selection Backspace still goes to the previous picture.
- **AI menu** with **Web search and fetch**: whether the chat's Claude may use the web (on by default; the choice is kept in `ai.json` in the settings folder).
- **Edit → Select All** (Cmd+A): all the text in the chat box (or the text being typed), the whole picture otherwise. Copy, paste, undo and redo of the Edit menu work in the chat box too.
- **Chat tools**: `apply_effect` (all the effects, and gradient, hue, gamma, levels, with strengths and a soft edge, on a box, polygon, mask or the user's selection), `clear_selection`, `resize_image`, `rotate_image` (any angle), `flip_image`, `place_image` (a picture from a file, scaled, see-through), `draw_polygon`, `draw_callout` (a speech bubble), rounded corners, opacity and shadows for shapes, `add_text` with alignment, wrapping, outline and shadow, `history` (the undo steps, by the user or Claude), `goto_step`, `checkpoint` and `goto_checkpoint`, `image_info` (file name without the folder, EXIF, how the colors are used, also for a part), `diff_original`, `list_folder`, `list_fonts`, `show_area`, and a JPEG quality for `save_image`.

### Changed
- Dragging a selection no longer depends on edit mode: a plain drag of its inside or an edge moves or resizes just the frame, and **Shift** and drag moves or stretches the selected picture itself (the place it came from turns white). A part that floats (a pasted picture, or one that was lifted with Shift) moves and stretches with a plain drag until it is applied.
- A right click no longer drops the selection; only a drag with the right button draws a new polygon.
- A copy to the clipboard no longer flashes the status bar: an orange clipboard appears at the end of the folder label for a moment, and the clipboard in the middle of the selection is orange too.
- The status bar has two groups, the file on the left and the readout and buttons on the right; when the window is too narrow the right group goes to a second row of its own.
- In edit mode a picture bigger than the window can be moved so that its whole left and bottom edge is in the open (not under the tool rail and the status bar), and the modes 4, 5 and 6 (stretch, fit width, fit height) fill only the visible part of the window.
- The chat button is shown in full screen when edit mode is on.
- The chat's Claude can use the web search and fetch tools as well as the picture tools; files and the shell stay off.

- A selection that can be moved is shown differently: blue dashes, a light blue tint and a small move sign in the corner, for a part that floats (it moves with a plain drag) and while Shift is held (the picture under the selection moves). The usual black and white dashes mean that only the frame moves.

### Fixed
- Paste no longer depends on edit mode, and no longer always starts another new image: a small clipboard picture is laid over the picture as a part (a selection) that can be moved, without turning edit mode on, and one about as big as the picture (or bigger) is asked about (new image, paste, or cancel). An empty window takes the clipboard picture as its picture.

## 0.37.8 – 2026-10-26

Everything since 0.31.0, in one release: a chat with Claude next to the picture, and the tools to remove things from a photo.

### Added
- **Chat panel** (the speech bubble at the top right of a picture window, or **C**): chat with Claude about the picture and ask it to change it. It runs the Claude Code that is installed on the computer, in the background, with the user's Claude subscription (no API key and no terminal). If the user is not signed in, the panel offers **Sign in with Claude**, which opens a browser tab; Claude Code keeps the sign-in and mcView never sees it. Claude gets the picture tools of the window and nothing else (no files, no shell, no web). Answers stream in and are shown formatted (Markdown); the tools it uses are one line (`Processing… · 14 actions · 1 failed`, a click opens the list); **Stop** ends an answer, **New** starts a new conversation. The panel slides in and out, is minimized with the bar button, can be resized by dragging its left border (the width is remembered), and its message box grows with what is typed. See `docs/chat.md`.
- **Picture tools for the chat**, from a private tool server of the window (it listens on `127.0.0.1` only while the chat is used, with a token that exists only inside the program, and serves that window only): `snapshot` (also zoomed in whole pixels, with an outline or a tinted mask, and the original file next to the picture now), the drawing tools (`draw_arrow`, `draw_line`, `draw_rect`, `draw_ellipse`, `draw_path`, `add_text`, `flood_fill`), `set_pixels`, `new_image`, `get_image`, `resize_canvas` and `crop`, selecting, copy and paste, and undo and redo. Every tool changes the picture at once; nothing moves a pointer or presses a key.
- **Retouching with Claude**: masks (`select_by_color` outlines an object by its color, `create_mask` takes polygons of any number of points, `edit_mask`, `selection_mask` for what the user selected), `clone_region` (flip, any angle, scale, soft edge, opacity, and a seamless healing-brush mode), `blend_edges` (smooth, color match, Poisson), `fill_region` (patches from the surroundings, or smooth, nearest, average, solid, with grain and with limits on where it may copy from), `seam_score`, `discard_changes`. Claude is told to trace letters and small details exactly and to remove them one by one, never with a rectangle, and what to do when the user marks part of the picture.
- **Polygon select** (the second tool of the edit rail): draw round the part freely; when the button is up the end is joined to the start and mcView turns the line into a polygon with a few corners, within about a pixel of what was drawn. **Hold the right mouse button** with the select tool or this tool to do the same. Delete fills the polygon with the second color, Ctrl+C copies what is inside it (the rest transparent), Esc lets it go. **Esc or a right click while a selection is being drawn drops it** (the rectangle too).
- **File → Discard Changes** and a button in the edit bar: the picture goes back to the file on the disk (or to the pasted or new picture) as a step of the history, so undo brings the changes back and redo discards them again.
- **Create image with your own settings**: Shift-click on *Create image* (or Shift+N) asks for the width, the height and the background (white, transparent or a color). The card says "Hold Shift for custom" and "Click to customize" while Shift is down. The plain click, N and File → Create Image still make the white 1280 × 720 picture.

- **The manual**, `docs/help.md`: a nice help for the app (looking at pictures, editing, selecting, the chat with Claude and what it sends, all the keys, what to do when something does not work), which *Help → mcView Help* shows on the website. The README has the polygon select, Discard Changes and the chat.

### Changed
- **Control mode** no longer simulates a mouse or a keyboard: its pointer and key commands (`move`, `click`, `dblclick`, `drag`, `release`, `key`, `press`) are gone, and `shape`, `fill`, `select`, `text`, `pixel`, `polygon` and others put their result in at once. `scripts/demo_screenshots.py` uses them.
- A picture that came from the clipboard is not "modified" until something is done to it, like a new blank image: no ● in the title, and closing it (Esc twice, Close Image, closing the window) or going to another picture asks nothing. The first change makes it modified, and undo back to it makes it unmodified again.
- File → Delete Image… is greyed out when there is nothing to delete (no picture, or a picture without a file), and Save As… and Close Image are greyed out without a picture.
- Double-clicking the picture no longer changes the zoom. Use **1** for 100%, **F** or **2** to fit, the wheel, a pinch, **+** and **-**.

### Fixed
- Closing a picture with transparency left the squares of the transparency on the home screen.

## 0.31.0 – 2026-10-26

Everything since 0.12.27, in one release: an editor next to the viewer.

### Added
- **Edit mode** (**E** or **Tab**): the title says `[edit]`, the bar says `EDIT`, the view is pixelated, and a tool rail with two color wells and a palette appears. It needs a picture (File → Create Image makes a new white 1280 × 720 one, which counts as unchanged until you do something to it).
- **Tools**, in the way of Paint, each with its options in a ribbon above the bottom bar: Select (move and stretch a selected part), **Pencil** (round, width 1 to 64), **Line**, **Rectangle** and **Circle/ellipse** (Shift: 45° line, square, circle; width; solid, dashed or dotted; outline, outline with fill, fill; the right button uses the second color; Esc or the other mouse button drops one that is being drawn), **Fill** (the paint bucket, with a tolerance), **Text** (font, size, bold, italic) and **Pick** (takes a color; the last tool comes back). Every one is a step of the undo history. The two colors have a palette that slides up, and the pixel readout in the bar shows x and y, R G B with meters and the hex color.
- **Selections** are put on the clipboard the moment you let go, with a short highlight (a clipboard fades in, the border and the message flash) and their size written in the middle while they are drawn. S saves just the selection as a picture of its own. A selection starts only when the drag covers more than one pixel.
- **Save and close**: S on a changed picture from a file asks **Overwrite**, **Save As** (the dialog offers the name with the date and time added, `photo-20261026-153012.jpg`) or **Cancel**. Closing a picture with changes asks Save, Discard or Cancel: **Ctrl/Cmd+W** (closes the picture, or the window when there is none), **Esc Esc** (two presses within 250 ms; on the home screen it asks whether to quit), the **✕** in edit mode, File → Close Image, going to another picture. **D** deletes the picture after a question (the Trash on a Mac) and shows the next one.
- **Home screen**: three big buttons, **Open file…** (**O**), **Open from clipboard** (Ctrl/Cmd+V) and **Create image** (**N**). **O** and **N** work with a picture open too, and the question about unsaved changes comes before the Open dialog.
- **Zoom modes 5 and 6**, as in IrfanView: the side edges touch the window, or the top and bottom edges. Mode 2 fits the picture to 85% of the window and is what every window starts with.
- **Paste as a new picture** when edit mode is off (undo brings the old one back).
- **Help → mcView Help** opens the manual (mormitech.com/tools/mcview/docs), with F1 on Windows and Linux; on macOS it is a Help menu after Window.
- **Control mode**: `mcview --control 0` lets another program drive every feature (menus, tools, shapes, fills, selections and text in picture pixels, zoom, options, full screen, the answers of the dialogs) over a local, token-protected connection. For demo screenshots and scripted tests; see `docs/control.md`, `scripts/mcview_ctl.py` and `scripts/demo_screenshots.py`. Off unless asked for.

### Changed
- The window title says the picture's size, its place in the folder and the zoom; the bottom bar has no status line in view mode and repeats them only in full screen (where there is no title).
- Full screen (**F** or **Enter**, also on the home screen) has a black background; Esc does not leave it, because Esc twice closes the picture. The bottom bar comes up when the pointer is at the bottom edge or while there is a selection. On Windows the menu bar is hidden in full screen and shows at the top edge.
- US English spelling ("color").

### Fixed
- The pencil stops the moment the mouse button is let go (it used to follow the pointer until the stroke was drawn), and no longer freezes the window.
- A stale clipboard highlight no longer reappears over the next selection.
- The pencil cursor shows only over the picture; the status bar keeps one height and never takes the pointer.
- On Windows the menu shortcuts Ctrl+O, Ctrl+W, Ctrl+Shift+W and Ctrl+Q also work while the menu bar is hidden in full screen.

## 0.12.27 – 2026-10-25

### Added
- **Installers for macOS and Linux.** A `.dmg` for macOS (open it and drag mcView to Applications), and a `.deb` (Debian, Ubuntu) and an `.rpm` (Fedora, openSUSE) for Linux, which also put mcView in the application menu and in Open With.
  The release workflow builds them and tests them (the disk image is opened and its app is checked; the .deb is installed and removed; the .rpm is checked for its files and requirements), so they come with every release from now on.
  No AppImage yet: it would have to carry WebKit with it.

## 0.12.26 – 2026-10-25

The steps 0.12.0 to 0.12.25 were never released one by one; this is everything since 0.11.3.

### Added
- **Undo and redo** (Ctrl+Z, Ctrl+Shift+Z / Ctrl+Y; Cmd+Z, Cmd+Shift+Z on a Mac, and Edit → Undo, Redo). Every change of what the window shows is a step: another picture (Open, the arrow keys, paste, save to another
  folder), the mode (1 to 4, F), the pixelated view (Tab), the turn (R), a moved or stretched part, a pasted layer, Close Image. A step brings the view back the way it was when it was made (the zoom, its mode, the part that is visible);
  zooming and panning themselves are not steps. Going back to the first state gives the empty window.
- **Rotate the view** with R (Shift+R the other way), a quick animation; it keeps the pixel that was in the middle in the middle, and so does full screen (Enter) and back. Only the view turns, until you save.
- **Select by dragging** with the mouse (no Shift needed); a click takes the selection away. The border is like Paint's: still dashes in black and dirty white while the button is down, "marching ants" once it is set; the selected part is a
  little lighter. With Shift, the edges of the frame move the frame only. The picture is panned with the middle mouse button (the pointer shows the grabbing hand), the trackpad and the wheel as before.
- **Move and stretch the selected part, like Paint.** Drag an edge or corner to stretch it, drag the inside (or the top left corner) to move it. The place it came from turns white (transparent in a picture that has transparency) and the part
  stays a layer until the selection is let go (Esc, a click beside it, a new selection, R, copy, paste, save, closing); then it is applied. Every drop and the applying are steps of undo. The picture is not recoded for this: it is kept as decoded pixels and only
  encoded again by Save As, and the color profile of the file (ICC of a JPEG or WebP, the color chunks of a PNG) is carried along, so the colors do not change when you edit.
- **Paste is a layer.** Ctrl+V (Cmd+V, Edit → Paste) lays the clipboard picture over the picture at its own size, on pixels near the top left corner of what is visible, selected and floating like a lifted part: drag it, stretch it, take it away with
  Delete, Backspace or Esc, or lay it down with a click. A clipboard picture that covers about half of the picture or more (or does not fit in it) is probably a new image, so mcView asks "Large image content is about to be pasted":
  Paste as new image, Paste into current image, Cancel. A window with no picture takes the clipboard picture as its picture. It works on JPEG, PNG, GIF, BMP, WebP and TIFF pictures.
- **A ● after the name of a modified picture** in the title and, in full screen, in the bottom bar.
- **A native alert before unsaved changes are lost.** Going to another picture or closing the window with a modified picture asks first, in the system's own alert: on macOS Save, Discard, Cancel; on Windows Yes, No, Cancel; on Linux GTK's two-button question
  twice. Save starts Save As and goes on once the file is written.
- **Transparent pictures get squares** (small bluish and white ones, as Gimp does) instead of a flat color. Save As offers such a picture as PNG, writes GIF with a transparent color, and says when JPEG made the transparent parts white.
- **File → Close Image** (Ctrl+Shift+W, Cmd+Shift+W): back to the way the window starts, with no picture; undo brings the picture back. **File → Close Window** (Cmd+W) on a Mac.
- **The window comes up where and how big it was left** (`window.json` in the settings folder of the user); full screen, maximised and minimised windows are not remembered, and a place that is no longer on a screen is ignored.
- **The edit bar** (key E) slides in a bar of tools from the left, like the bottom bar in full screen; Line, Rectangle and Text are there but do nothing yet.
- **In full screen the bottom bar is hidden** until Shift is held, slides and fades in and out, and shows the file name.
- With MCVIEW_TRACE set, a failure of the page shows in the trace.

### Changed
- **Save As saves what you see:** the picture as it is turned and with what was pasted or moved, starting in the folder of the picture (the Pictures folder for one from the clipboard). The saved file is the picture of the window afterwards. A picture of a type mcView does not write
  (WebP, SVG…) is offered as PNG.
- The next or the previous picture is decoded off screen and takes the place of the old one in one step, so nothing flickers; the same for an edit that is applied.

### Fixed
- Windows: the installer registered the "Open with" entries with values that were never created (fixed in 0.11.3 as well).
- A selection that mcView made itself (a pasted picture, a moved part, the selection after an undo) was one pixel too big in the pixelated view.
- The dashed border of a selection left traces behind while it was dragged.
- The circle of the title no longer goes out for a moment on undo and redo of a moved part, and undo no longer flashes the untouched picture.

## 0.11.3 – 2026-10-25

### Fixed
- **Windows: "Open with…" did not list mcView.** The installer wrote the picture types as registry values of type "none", which Inno Setup does not
  create at all; they are empty strings now. (Found by the installer test of the first GitHub build.)

## 0.11.2 – 2026-10-25

### Changed
- **The file name is in the bar in full screen**, with its extension, at the left (the title bar that normally says it is not there).

## 0.11.1 – 2026-10-25

### Fixed
- **Cmd+W did nothing on a Mac.** File → Close Window (Cmd+W) closes the window, like the red button does (which ends mcView, as it is one window per
  program). In the About window Cmd+W (Ctrl+W elsewhere) and Esc close only that window.

## 0.11.0 – 2026-10-25

### Added
- **Marching ants on the copied selection.** After Ctrl+C (Cmd+C) the frame of the selection that was copied is drawn as a dashed line that
  moves, to show that this part is on the clipboard. It stays so until the frame is changed or moved; clearing the selection (a click, Esc) does not
  touch the clipboard, the frame is just not marked any more.

## 0.10.2 – 2026-10-25

### Changed
- **The bar in full screen slides and fades** in and out (0.12 s) instead of appearing at once.

## 0.10.1 – 2026-10-25

### Changed
- **No bar at the bottom in full screen.** The bar with the folder, the picture's place and size and the pixel readout is hidden in full screen,
  and shown for as long as Shift is held (a message, like "Copied …", also shows it for a few seconds).

## 0.10.0 – 2026-10-25

### Added
- **Resize the selection.** With a selection, hold Shift and move over one of its edges (or a corner): the edge lights up blue and the pointer
  becomes a resize arrow. Shift and drag then moves that edge (a corner moves two), in the pixelated view snapped to whole pixels.

## 0.9.1 – 2026-10-25

### Changed
- **Full screen (Enter) and back keep the middle.** When the picture is zoomed or moved, the pixel that was in the middle of the window is in the
  middle of the full screen, and again in the middle of the window when you leave it. This holds for any resize of the window. A picture that
  cannot be held in the middle (it would show a gap at its edge) stays at its edge, and comes back to the middle when there is room again.

## 0.9.0 – 2026-10-25

### Added
- **Undo and redo.** Ctrl+Z (Cmd+Z on a Mac) undoes, Ctrl+Shift+Z (Cmd+Shift+Z; also Ctrl+Y) redoes, and Edit → Undo / Redo do the same. Undoable are every
  change of what the window shows: opening a picture (also from the file manager, and from an empty window back to empty), the arrow keys
  and Space/Backspace through the folder, pasting, saving to another folder, the modes 1 to 4 and F, the pixelated view (Tab) and the turn (R).
  Not steps: how far it is zoomed, where it is moved, the selection. The history keeps 100 steps; a pasted picture stays in it, so
  undoing a paste brings the picture before it back, and redoing brings the pasted one back.

## 0.8.1 – 2026-10-25

### Changed
- **Turning a zoomed picture keeps the zoom and the middle.** When the picture was zoomed or moved, R and Shift+R no longer go back to the
  mode's fit: the scale stays, and the pixel that was in the middle of the window is in the middle after the turn. A picture that
  the mode (1 to 4) places is still fitted to the turned picture.

## 0.8.0 – 2026-10-25

### Added
- **Rotate the view.** **R** turns the picture a quarter clockwise, **Shift+R** a quarter counterclockwise, with a quick 0.2 s animation. It is only the
  view: no file is changed, and the next picture is shown upright again. The mode (1 to 4) fits the turned picture to the window; the
  size in the bar, the selection (cleared by a turn), the pixel readout (positions as on the screen) and Ctrl+C (the copy is turned the way it is shown)
  all follow the turned picture.

## 0.7.3 – 2026-10-25

### Changed
- **The bounce at the edge is a spring now, and comes back at once.** After the fingers have left, the trackpad keeps sending scroll events for
  about a second, each smaller than the one before (it lets the movement coast), and the old bounce waited for them to stop, so the
  picture stayed out long after the swipe was over. Now a spring takes the pull away every frame, and events that only get smaller do
  not push the picture out again: in a test with a swipe followed by 0.6 s of coasting, the picture was back **275 ms** after the last
  push (it took 600 ms and more, with a second small bounce, before).
- **A picture that fits the window bounces too** when you swipe or drag it, so that it is plain there is nowhere to pan.

## 0.7.2 – 2026-10-25

### Added
- **Bounce at the edge.** A picture bigger than the window can be pulled a little past its edge (up to 20 pixels, and the harder the further
  it goes) with the mouse button held or two fingers on the trackpad, and eases back when it is let go. A picture that fits the window on an
  axis does not move on that axis.

### Fixed
- **Ghost lines under or beside the picture after zooming out fast.** One-pixel lines of the old edges stayed on the screen, because the
  picture had fractional sizes and positions and the web view did not repaint the sliver at the old edge. The picture now has whole
  pixel sizes and positions. In a test that zooms out from 300% to the minimum with a burst of wheel events, 8 of 8 windows had
  stale lines before and none has now.

## 0.7.1 – 2026-10-25

### Added
- **Ctrl+N opens a new window** on every system, as well as in the File menu. On Windows and Linux that is the usual key; on a Mac the menu
  has Cmd+N and Ctrl+N works too. Both together never open two windows.

## 0.7.0 – 2026-10-25

### Added
- **File → New Window** (Cmd+N, Ctrl+N) opens an empty window, a little down and to the right of the one it came from.
- **Paste a picture from the clipboard** (Cmd+V, Ctrl+V, Shift+Insert, Edit → Paste): the window shows it. The title says
  `[clipboard image]` instead of a file name, and the bar shows the Pictures folder of the user. The pixel readout, the selection and
  Ctrl+C work on it. The arrow keys do nothing but say so while the picture is not saved, so that it is not lost by accident, and a file
  opened from the system gets a window of its own instead of replacing it.
- **S saves the picture** (also Cmd+S, Ctrl+S and File → Save As…) in a dialog that starts in the folder of the picture (the Pictures
  folder for a pasted one). The type follows the name: PNG, JPEG, GIF, BMP or TIFF; a name without one gets `.png`. A file saved as the
  type it already is is copied byte for byte; any other is decoded and written again, turned the way it is shown. A pasted picture
  becomes the file it was saved to, and **saving to another folder makes that folder the one the window works in**: the bar shows it,
  and the arrow keys go through its pictures.
- An **Edit menu** with Copy and Paste on every system (the system's own work on text in the page, not on a picture).

### Changed
- The first window is the same as before; the hint in an empty window now mentions pasting.

## 0.6.0 – 2026-10-25

### Added
- **Select a part of the picture: hold Shift and drag.** The selection is a white frame with a soft dark shadow, so it shows on a white
  picture as well. In the pixelated view it snaps to whole pixels; in the smoothed view it follows the pointer, and the pixels it touches
  are selected. It stays on its place when the picture is zoomed or moved. A click on the picture, Esc, or another picture takes it away.
  The bar shows its size (`selection 120 × 80`).
- **Ctrl+C (Cmd+C on a Mac, and Ctrl+Insert) copies the selected part to the clipboard** as a picture; with nothing selected it copies the
  whole picture. The part is cut out of the decoded file, so it has the colors of the file (transparency included). The web view's own
  clipboard is tried first, then the system's way (osascript on macOS, PowerShell on Windows, `xclip` or `wl-copy` on Linux, which have to
  be installed). AVIF, SVG and ICO are cut from what the web view drew.

### Changed
- The pixel readout reads `[x, y]  (r, g, b) - #rrggbb`, with the alpha as a fourth number and at the end of the hex code when the pixel
  is not opaque.

## 0.5.0 – 2026-10-25

### Added
- **Tab switches between the smoothed view and the pixelated view**, where every pixel of the picture is a sharp square and nothing is
  blended. Tab again goes back. The pixelated view stays when the next or the previous picture is shown, but not the next time mcView
  starts (it always starts smoothed). The bar says `pixels` while it is on.
- **Pixel readout.** In the pixelated view the bottom bar shows the color of the pixel under the pointer: its position, R G B and
  alpha (0 to 255), the hex code and a swatch (a checkerboard shows through a transparent pixel).
  - The numbers are those of the file: mcView decodes the picture a second time in Go, without any color conversion (the web view
    converts colors for the screen, and its JPEG decoder can be one off in the last digit). JPEG, PNG, GIF, BMP, WebP and TIFF.
    A JPEG turned by its EXIF Orientation tag is read the way it is shown.
  - For types Go cannot decode (AVIF, SVG, ICO) the color is read from what the web view drew, and the bar says `from the screen`.
  - A picture over 150 megapixels is not decoded for the readout.

## 0.4.1 – 2026-10-25

### Added
- **Space** shows the next picture of the folder, like the right arrow, and **Backspace** the previous one, like the left arrow.

## 0.4.0 – 2026-10-25

### Added
- **Enter** toggles full screen, and **Esc** leaves it. On macOS this is the system's full screen, in a Space of its own.

## 0.3.0 – 2026-10-25

### Added
- **Zoom modes on the keys 1 to 4**, the way IrfanView has them: **1** original size (100%, even when the picture is small or bigger than
  the window), **2** big pictures are shrunk to fit and small ones stay 100% (the default, and what **F** does), **3** every picture
  is zoomed up or down to touch the window borders, **4** every picture is stretched to the window borders. The mode stays when the
  next or the previous picture is shown, and the next time mcView starts. Zooming or moving with the mouse leaves the mode until the
  next picture. In the stretch mode the bar shows both zooms (`34% × 91%`).
- **Mac trackpad:** two fingers up and down (or sideways) move the picture instead of zooming it. A mouse wheel still zooms, and so does a
  pinch. The two are told apart by the size of the scroll steps.

### Changed
- **Left and right arrow go round**: after the last picture of the folder comes the first (8/8 → 1/8), and before the first comes the last.
  The "last picture" message is gone.

## 0.2.0 – 2026-10-25

### Added
- **Left and right arrow** show the previous and the next picture of the folder of the current one, in alphabetical order (upper and
  lower case alike). At the first or the last picture the bar says so and nothing moves.
- The bottom bar shows the folder of the picture (the name is in the title), and on the right which picture of the folder it is
  (`2 / 3`), its size in pixels and the zoom.
- `MCVIEW_TRACE=<file>` writes timestamps of the start and of showing a picture to that file, for tracking down slow starts.

### Fixed
- **A picture opened with Open With, a double-click or `open -a` took about 5 seconds to show.** macOS starts an app hidden when it is
  launched with a file, and Wails did not unhide it, so the window (and the picture) came up only after a timeout. The window is now
  shown at once: a 36 megapixel JPEG is on the screen in under a second, a small one in about 0.4 s.
- The hand ("grab") cursor over the picture is gone. It was also showing over the title bar buttons.

### Changed
- The picture is shown at its size on the screen instead of scaled with a CSS transform.

## 0.1.0 – 2026-10-25

### Added
- **mcView shows pictures.** JPEG (jpg, jpeg, jpe, jfif), PNG, GIF, BMP, WebP, ICO, SVG, AVIF and TIFF. The picture is fitted to the window;
  scroll to zoom, drag to move, double-click for 100% and back, **F** fits, **1** is 100%, **+** and **-** zoom. The window title and a
  status line show the name, the size in pixels and the zoom.
- **Open with…** offers mcView for these file types, with no signing needed.
  - macOS: the app declares the types in its Info.plist. It is offered in Open With and never takes a type over by itself;
    choose it in Get Info → Open with → Change All to make it the default.
  - Windows: the installer adds the types per user (no admin rights); mcView is in Open with… and in Settings → Apps → Default apps.
    Windows does not let an installer make a program the default.
  - Linux: `install.sh` in the tar.gz puts a desktop entry in `~/.local/share/applications`.
- File → Open… (⌘O, Ctrl+O). A second picture opened from the file manager gets a window of its own.

### Changed
- The Hello World window is the empty state of the viewer.

## 0.0.1 – 2026-10-25

### Added
- The app stub: a window that says Hello, world. No functionality yet.
- An About window (About mcView in the application menu on macOS, Help → About mcView on Windows and Linux) with the Mormitech logo,
  the version and links.
- The window is cut down to 90% of the screen at start and centred, so its title bar stays reachable with a high scaling setting.
- `--about` and `--theme light|dark` command line options.
- Builds and tests on macOS, Windows and Linux, and a Windows installer (Inno Setup).
