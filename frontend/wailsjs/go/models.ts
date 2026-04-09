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
	export class EmbeddingModelConfig {
	    provider: string;
	    modelId: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingModelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.modelId = source["modelId"];
	        this.enabled = source["enabled"];
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
	export class ManagedModelDownloadState {
	    key: string;
	    kind: string;
	    modelId: string;
	    displayName: string;
	    active: boolean;
	    finished: boolean;
	    success: boolean;
	    status: string;
	    message: string;
	    currentFile: string;
	    filesCompleted: number;
	    filesTotal: number;
	    bytesDownloaded: number;
	    bytesTotal: number;
	    progressPct: number;
	
	    static createFrom(source: any = {}) {
	        return new ManagedModelDownloadState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.modelId = source["modelId"];
	        this.displayName = source["displayName"];
	        this.active = source["active"];
	        this.finished = source["finished"];
	        this.success = source["success"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.currentFile = source["currentFile"];
	        this.filesCompleted = source["filesCompleted"];
	        this.filesTotal = source["filesTotal"];
	        this.bytesDownloaded = source["bytesDownloaded"];
	        this.bytesTotal = source["bytesTotal"];
	        this.progressPct = source["progressPct"];
	    }
	}
	export class ManagedModelStatus {
	    key: string;
	    kind: string;
	    displayName: string;
	    provider: string;
	    modelId: string;
	    backend?: string;
	    installed: boolean;
	    active: boolean;
	    status: string;
	    message: string;
	    installDir: string;
	    downloadUrl?: string;
	    canDownload: boolean;
	    downloading: boolean;
	    currentFile?: string;
	    progressPct?: number;
	    progressText?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedModelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.kind = source["kind"];
	        this.displayName = source["displayName"];
	        this.provider = source["provider"];
	        this.modelId = source["modelId"];
	        this.backend = source["backend"];
	        this.installed = source["installed"];
	        this.active = source["active"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.installDir = source["installDir"];
	        this.downloadUrl = source["downloadUrl"];
	        this.canDownload = source["canDownload"];
	        this.downloading = source["downloading"];
	        this.currentFile = source["currentFile"];
	        this.progressPct = source["progressPct"];
	        this.progressText = source["progressText"];
	    }
	}
	export class WelcomeState {
	    showModal: boolean;
	    requiresMigration: boolean;
	    migrationMessage: string;
	    requiresTtsDownload: boolean;
	    ttsDownloadMessage: string;
	    canSkipTtsDownload: boolean;
	    restartRecommended: boolean;
	    primaryMessage: string;
	    secondaryMessage: string;
	
	    static createFrom(source: any = {}) {
	        return new WelcomeState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.showModal = source["showModal"];
	        this.requiresMigration = source["requiresMigration"];
	        this.migrationMessage = source["migrationMessage"];
	        this.requiresTtsDownload = source["requiresTtsDownload"];
	        this.ttsDownloadMessage = source["ttsDownloadMessage"];
	        this.canSkipTtsDownload = source["canSkipTtsDownload"];
	        this.restartRecommended = source["restartRecommended"];
	        this.primaryMessage = source["primaryMessage"];
	        this.secondaryMessage = source["secondaryMessage"];
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

}

export namespace mcp {
	
	export class MemoryRetentionConfig {
	    coreDays: number;
	    workingDays: number;
	    ephemeralDays: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryRetentionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.coreDays = source["coreDays"];
	        this.workingDays = source["workingDays"];
	        this.ephemeralDays = source["ephemeralDays"];
	    }
	}

}

export namespace promptkit {
	
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
