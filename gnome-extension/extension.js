// GNOME 45+ extension entry point (ESModules).
import * as Main from 'resource:///org/gnome/shell/ui/main.js';
import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';
import { TokenEaterIndicator } from './panel.js';

export default class TokenEaterExtension extends Extension {
    enable() {
        this._indicator = new TokenEaterIndicator(this);
        Main.panel.addToStatusArea(this.uuid, this._indicator);
    }

    disable() {
        if (this._indicator) {
            this._indicator.destroy();
            this._indicator = null;
        }
    }
}
