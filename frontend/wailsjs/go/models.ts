export namespace config {
	
	export class HistoryItem {
	    text: string;
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class Config {
	    api_key: string;
	    shortcut: string;
	    whisper_model: string;
	    ai_model: string;
	    ai_prompt: string;
	    language: string;
	    microphone_device?: number;
	    history: HistoryItem[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_key = source["api_key"];
	        this.shortcut = source["shortcut"];
	        this.whisper_model = source["whisper_model"];
	        this.ai_model = source["ai_model"];
	        this.ai_prompt = source["ai_prompt"];
	        this.language = source["language"];
	        this.microphone_device = source["microphone_device"];
	        this.history = this.convertValues(source["history"], HistoryItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

