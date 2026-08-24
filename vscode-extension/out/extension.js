"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = __importStar(require("vscode"));
const net = __importStar(require("node:net"));
const path = __importStar(require("node:path"));
const node_1 = require("vscode-languageclient/node");
let client;
async function activate(context) {
    const outputChannel = vscode.window.createOutputChannel('eirctl Language Server', { log: true });
    context.subscriptions.push(outputChannel);
    if (context.extensionMode === vscode.ExtensionMode.Development) {
        outputChannel.show(true);
    }
    const serverOptions = getServerOptions(context, outputChannel);
    outputChannel.info('Activating eirctl extension');
    outputChannel.info(`Extension mode: ${vscode.ExtensionMode[context.extensionMode]}`);
    const clientOptions = {
        documentSelector: [
            { language: 'yaml', pattern: '**/eirctl.yaml' },
            { language: 'yaml', pattern: '**/eirctl/*.yaml' },
            { language: 'yaml', pattern: '**/.eirctl/cache/*' },
        ],
        outputChannel,
        traceOutputChannel: outputChannel,
        synchronize: {
            configurationSection: 'eirctl',
            fileEvents: [
                vscode.workspace.createFileSystemWatcher('**/eirctl.yaml'),
                vscode.workspace.createFileSystemWatcher('**/eirctl/**/*.yaml')
            ],
        },
    };
    client = new node_1.LanguageClient('eirctl', 'eirctl Language Server', serverOptions, clientOptions);
    context.subscriptions.push({ dispose: () => void client?.stop() });
    context.subscriptions.push(client.onDidChangeState((event) => {
        outputChannel.info(`Language client state changed: ${event.oldState} -> ${event.newState}`);
    }));
    try {
        await client.start();
        outputChannel.info('eirctl language client started');
    }
    catch (error) {
        outputChannel.error(String(error));
        void vscode.window.showErrorMessage('Failed to start the eirctl language server. Check the eirctl Language Server output channel.');
        throw error;
    }
}
async function deactivate() {
    if (!client) {
        return;
    }
    await client.stop();
    client = undefined;
}
function getServerExecutable(context) {
    const config = vscode.workspace.getConfiguration('eirctl');
    const defaultCwd = path.resolve(context.extensionPath, '..');
    const configuredCwd = config.get('languageServer.cwd');
    const command = config.get('languageServer.command', 'go');
    const args = config.get('languageServer.args', ['run', '/home/dnitsch/git/ensono/eirctl/cmd/eirctl-lsp/main.go']);
    return {
        command,
        args,
        options: {
            cwd: configuredCwd && configuredCwd.trim() !== '' ? configuredCwd : defaultCwd,
            env: {
                ...process.env,
            },
        },
    };
}
function getServerOptions(context, outputChannel) {
    const config = vscode.workspace.getConfiguration('eirctl');
    const transport = config.get('languageServer.transport', 'process');
    if (transport === 'tcp') {
        return getServerSocket(outputChannel, config);
    }
    const executable = getServerExecutable(context);
    outputChannel.info('Server transport: process');
    outputChannel.info(`Server command: ${executable.command} ${(executable.args ?? []).join(' ')}`);
    outputChannel.info(`Server cwd: ${executable.options.cwd ?? '<unset>'}`);
    return executable;
}
function getServerSocket(outputChannel, config) {
    const host = config.get('languageServer.tcpHost', '127.0.0.1');
    const port = config.get('languageServer.tcpPort', 11103);
    outputChannel.info(`Server transport: tcp (${host}:${port})`);
    return () => new Promise((resolve, reject) => {
        const socket = net.createConnection(port, host, () => {
            resolve({ reader: socket, writer: socket });
        });
        socket.once('error', (error) => {
            socket.destroy();
            reject(error);
        });
    });
}
//# sourceMappingURL=extension.js.map