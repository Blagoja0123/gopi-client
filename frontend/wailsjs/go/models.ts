export namespace grpcrunner {
	
	export class MethodInfo {
	    name: string;
	    fullName: string;
	    inputType: string;
	    outputType: string;
	    clientStreaming: boolean;
	    serverStreaming: boolean;
	    template: string;
	
	    static createFrom(source: any = {}) {
	        return new MethodInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.inputType = source["inputType"];
	        this.outputType = source["outputType"];
	        this.clientStreaming = source["clientStreaming"];
	        this.serverStreaming = source["serverStreaming"];
	        this.template = source["template"];
	    }
	}
	export class ServiceInfo {
	    name: string;
	    methods: MethodInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ServiceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.methods = this.convertValues(source["methods"], MethodInfo);
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

export namespace httprunner {
	
	export class Response {
	    status: number;
	    statusText: string;
	    proto: string;
	    headers: model.KV[];
	    contentType: string;
	    body: string;
	    bodyBase64: boolean;
	    size: number;
	    truncated: boolean;
	    durationMs: number;
	    url: string;
	    method: string;
	    requestHeaders: model.KV[];
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.statusText = source["statusText"];
	        this.proto = source["proto"];
	        this.headers = this.convertValues(source["headers"], model.KV);
	        this.contentType = source["contentType"];
	        this.body = source["body"];
	        this.bodyBase64 = source["bodyBase64"];
	        this.size = source["size"];
	        this.truncated = source["truncated"];
	        this.durationMs = source["durationMs"];
	        this.url = source["url"];
	        this.method = source["method"];
	        this.requestHeaders = this.convertValues(source["requestHeaders"], model.KV);
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

export namespace main {
	
	export class WorkspaceInfo {
	    root: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.name = source["name"];
	    }
	}
	export class AppState {
	    workspace?: WorkspaceInfo;
	    environments: string[];
	    activeEnv: string;
	    recent: string[];
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceInfo);
	        this.environments = source["environments"];
	        this.activeEnv = source["activeEnv"];
	        this.recent = source["recent"];
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
	export class Preview {
	    url: string;
	    missing: string[];
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Preview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.missing = source["missing"];
	        this.error = source["error"];
	    }
	}
	export class RunResult {
	    response?: httprunner.Response;
	    missing: string[];
	
	    static createFrom(source: any = {}) {
	        return new RunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.response = this.convertValues(source["response"], httprunner.Response);
	        this.missing = source["missing"];
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
	export class Signature {
	    header: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Signature(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.header = source["header"];
	        this.value = source["value"];
	    }
	}
	export class WebhookStatus {
	    running: boolean;
	    addr: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.addr = source["addr"];
	    }
	}

}

export namespace model {
	
	export class Auth {
	    type: string;
	    token?: string;
	    username?: string;
	    password?: string;
	    key?: string;
	    value?: string;
	    in?: string;
	
	    static createFrom(source: any = {}) {
	        return new Auth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.token = source["token"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.in = source["in"];
	    }
	}
	export class KV {
	    key: string;
	    value: string;
	    disabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KV(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.disabled = source["disabled"];
	    }
	}
	export class Body {
	    type: string;
	    content?: string;
	    form?: KV[];
	
	    static createFrom(source: any = {}) {
	        return new Body(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.content = source["content"];
	        this.form = this.convertValues(source["form"], KV);
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
	export class Variable {
	    key: string;
	    value: string;
	    secret?: boolean;
	    disabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Variable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.secret = source["secret"];
	        this.disabled = source["disabled"];
	    }
	}
	export class Environment {
	    name: string;
	    variables: Variable[];
	
	    static createFrom(source: any = {}) {
	        return new Environment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.variables = this.convertValues(source["variables"], Variable);
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
	export class Folder {
	    headers?: KV[];
	    auth?: Auth;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.headers = this.convertValues(source["headers"], KV);
	        this.auth = this.convertValues(source["auth"], Auth);
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
	export class GRPC {
	    service?: string;
	    method?: string;
	    protoFiles?: string[];
	    importPaths?: string[];
	    plaintext?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GRPC(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.service = source["service"];
	        this.method = source["method"];
	        this.protoFiles = source["protoFiles"];
	        this.importPaths = source["importPaths"];
	        this.plaintext = source["plaintext"];
	    }
	}
	
	export class Node {
	    name: string;
	    path: string;
	    isFolder: boolean;
	    kind?: string;
	    method?: string;
	    children?: Node[];
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isFolder = source["isFolder"];
	        this.kind = source["kind"];
	        this.method = source["method"];
	        this.children = this.convertValues(source["children"], Node);
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
	export class Settings {
	    timeoutMs?: number;
	    followRedirects?: boolean;
	    insecureSkipVerify?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeoutMs = source["timeoutMs"];
	        this.followRedirects = source["followRedirects"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	    }
	}
	export class WSMessage {
	    name: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new WSMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class WS {
	    subprotocols?: string[];
	    messages?: WSMessage[];
	
	    static createFrom(source: any = {}) {
	        return new WS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subprotocols = source["subprotocols"];
	        this.messages = this.convertValues(source["messages"], WSMessage);
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
	export class Request {
	    name: string;
	    kind: string;
	    method?: string;
	    url: string;
	    params?: KV[];
	    headers?: KV[];
	    auth?: Auth;
	    body?: Body;
	    grpc?: GRPC;
	    ws?: WS;
	    settings?: Settings;
	    docs?: string;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.params = this.convertValues(source["params"], KV);
	        this.headers = this.convertValues(source["headers"], KV);
	        this.auth = this.convertValues(source["auth"], Auth);
	        this.body = this.convertValues(source["body"], Body);
	        this.grpc = this.convertValues(source["grpc"], GRPC);
	        this.ws = this.convertValues(source["ws"], WS);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.docs = source["docs"];
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

export namespace scratch {
	
	export class Tab {
	    id: string;
	    request?: model.Request;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.request = this.convertValues(source["request"], model.Request);
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

export namespace webhook {
	
	export class Header {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Header(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class Capture {
	    id: string;
	    // Go type: time
	    time: any;
	    method: string;
	    path: string;
	    query: string;
	    headers: Header[];
	    body: string;
	    bodyBase64: boolean;
	    size: number;
	    remoteAddr: string;
	
	    static createFrom(source: any = {}) {
	        return new Capture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = this.convertValues(source["time"], null);
	        this.method = source["method"];
	        this.path = source["path"];
	        this.query = source["query"];
	        this.headers = this.convertValues(source["headers"], Header);
	        this.body = source["body"];
	        this.bodyBase64 = source["bodyBase64"];
	        this.size = source["size"];
	        this.remoteAddr = source["remoteAddr"];
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
	
	export class Response {
	    status: number;
	    contentType: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.contentType = source["contentType"];
	        this.body = source["body"];
	    }
	}

}

