import * as vscode from 'vscode';
import * as net from 'node:net';
import { resolve } from 'node:path';
import { LanguageClient, LanguageClientOptions, ServerOptions, State, StreamInfo } from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
    const outputChannel = vscode.window.createOutputChannel('eirctl Language Server', { log: true });
    context.subscriptions.push(outputChannel);
    if (context.extensionMode === vscode.ExtensionMode.Development) {
        outputChannel.show(true);
    }

    const serverOptions = getServerOptions(context, outputChannel);
    outputChannel.info('Activating eirctl extension');
    outputChannel.info(`Extension mode: ${vscode.ExtensionMode[context.extensionMode]}`);

    const clientOptions: LanguageClientOptions = {
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

    client = new LanguageClient('eirctl', 'eirctl Language Server', serverOptions, clientOptions);
    context.subscriptions.push({ dispose: () => void client?.stop() });
    context.subscriptions.push(client.onDidChangeState((event) => {
        outputChannel.info(`Language client state changed: ${State[event.oldState]} -> ${State[event.newState]}`);
    }));

    try {
        await client.start();
        outputChannel.info('eirctl language client started');
    } catch (error) {
        outputChannel.error(String(error));
        void vscode.window.showErrorMessage('Failed to start the eirctl language server. Check the eirctl Language Server output channel.');
        throw error;
    }
}

export async function deactivate(): Promise<void> {
    if (!client) {
        return;
    }
    await client.stop();
    client = undefined;
}

function getServerExecutable(context: vscode.ExtensionContext) {
    const config = vscode.workspace.getConfiguration('eirctl');
    const defaultCwd = resolve(context.extensionPath, '.bin');
    const configuredCwd = config.get<string>('languageServer.cwd');
    const command = config.get<string>('languageServer.command', '');
    const args = config.get<string[]>('languageServer.args', []);

    const fullCommand = resolve(configuredCwd && configuredCwd.trim() !== '' ? configuredCwd : defaultCwd, command);
    return {
        command: fullCommand,
        args,
        options: {
            cwd: configuredCwd && configuredCwd.trim() !== '' ? configuredCwd : defaultCwd,
            env: {
                ...process.env,
            },
        },
    };
}

function getServerOptions(context: vscode.ExtensionContext, outputChannel: vscode.LogOutputChannel): ServerOptions {
    const config = vscode.workspace.getConfiguration('eirctl');
    const transport = config.get<string>('languageServer.transport', 'process');

    if (transport === 'tcp') {
        return getServerSocket(outputChannel, config);
    }

    const executable = getServerExecutable(context);
    outputChannel.info('Server transport: process');
    outputChannel.info(`Server command: ${executable.command} ${(executable.args ?? []).join(' ')}`);
    outputChannel.info(`Server cwd: ${executable.options.cwd ?? '<unset>'}`);
    return executable;
}

function getServerSocket(outputChannel: vscode.LogOutputChannel, config: vscode.WorkspaceConfiguration): () => Promise<StreamInfo> {
    const host = config.get<string>('languageServer.tcpHost', '127.0.0.1');
    const port = config.get<number>('languageServer.tcpPort', 11103);

    outputChannel.info(`Server transport: tcp (${host}:${port})`);

    return () => new Promise<StreamInfo>((resolve, reject) => {
        const socket = net.createConnection(port, host, () => {
            resolve({ reader: socket, writer: socket });
        });

        socket.once('error', (error) => {
            socket.destroy();
            reject(error);
        });
    });
}
