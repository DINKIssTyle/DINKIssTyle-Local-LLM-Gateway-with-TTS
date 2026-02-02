export namespace main {
	
	export class HealthCheckResult {
	    llmStatus: string;
	    llmMessage: string;
	    ttsStatus: string;
	    ttsMessage: string;
	    serverModel: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.llmStatus = source["llmStatus"];
	        this.llmMessage = source["llmMessage"];
	        this.ttsStatus = source["ttsStatus"];
	        this.ttsMessage = source["ttsMessage"];
	        this.serverModel = source["serverModel"];
	    }
	}
	export class ServerTTSConfig {
	    voiceStyle: string;
	    speed: number;
	    threads: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerTTSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.voiceStyle = source["voiceStyle"];
	        this.speed = source["speed"];
	        this.threads = source["threads"];
	    }
	}
	export class SystemPrompt {
	    title: string;
	    prompt: string;
	
	    static createFrom(source: any = {}) {
	        return new SystemPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.prompt = source["prompt"];
	    }
	}

}

