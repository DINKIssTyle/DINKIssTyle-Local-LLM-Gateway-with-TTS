export namespace main {
	
	export class ServerTTSConfig {
	    voiceStyle: string;
	    speed: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerTTSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.voiceStyle = source["voiceStyle"];
	        this.speed = source["speed"];
	    }
	}

}

