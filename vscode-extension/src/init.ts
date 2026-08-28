import * as net from 'node:net'
import { resolve } from 'node:path'
import * as vscode from 'vscode'
import { ServerOptions, StreamInfo } from 'vscode-languageclient/node'


export function getServerExecutable(context: vscode.ExtensionContext) {
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

export function getServerOptions(context: vscode.ExtensionContext, outputChannel: vscode.LogOutputChannel): ServerOptions {
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

export function getServerSocket(outputChannel: vscode.LogOutputChannel, config: vscode.WorkspaceConfiguration): () => Promise<StreamInfo> {
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
