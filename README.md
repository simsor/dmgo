# dmgo - a gameboy emulator in go -- ported to the Kindle 4!
#### Features:
 * Saved game support!
 * All major [MBCs](http://gbdev.gg8.se/wiki/articles/Memory_Bank_Controllers) suppported!
 * Glitches are relatively rare but still totally happen!
 * The main bottleneck seems to be the screen redisplay function


#### Compile instructions

 * Build with `GOOS=linux`, `GOARCH=arm` and `GOARM=7`

#### Important Notes:

 * Keybindings are currently hardcoded to:
   * Kindle `DPad`: direction keys
   * `Back`: B
   * `Keyboard`: A
   * `Previous page` or `Next page` on the LEFT side: Select
   *  `Previous page` or `Next page` on the RIGHT side: Start
 * Saved games use/expect a slightly different naming convention than usual: romfilename.gb.sav
