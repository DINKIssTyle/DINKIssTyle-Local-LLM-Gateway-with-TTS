export namespace core {
	
	export class DebugTraceEntry {
	    id: number;
	    timestamp: string;
	    source: string;
	    stage: string;
	    message: string;
	    details?: Record<string, string>;
	    payload?: string;
	
	    static createFrom(source: any = {}) {
	        return new DebugTraceEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.source = source["source"];
	        this.stage = source["stage"];
	        this.message = source["message"];
	        this.details = source["details"];
	        this.payload = source["payload"];
	    }
	}
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
	    engine: string;
	    voiceStyle: string;
	    speed: number;
	    threads: number;
	    osVoiceURI?: string;
	    osVoiceName?: string;
	    osVoiceLang?: string;
	    osRate?: number;
	    osPitch?: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerTTSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.voiceStyle = source["voiceStyle"];
	        this.speed = source["speed"];
	        this.threads = source["threads"];
	        this.osVoiceURI = source["osVoiceURI"];
	        this.osVoiceName = source["osVoiceName"];
	        this.osVoiceLang = source["osVoiceLang"];
	        this.osRate = source["osRate"];
	        this.osPitch = source["osPitch"];
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

