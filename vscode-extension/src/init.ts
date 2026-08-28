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
    const maxRetries = config.get<number>('languageServer.tcpRetryAttempts', 5);
    const retryDelayMs = config.get<number>('languageServer.tcpRetryDelayMs', 300);
    
    outputChannel.info(`Server transport: tcp (${host}:${port})`);

    return () => connectWithRetry(host, port, maxRetries, retryDelayMs, outputChannel);
}

async function connectWithRetry(
    host: string,
    port: number,
    maxRetries: number,
    retryDelayMs: number,
    outputChannel: vscode.LogOutputChannel,
    attempt = 1
): Promise<StreamInfo> {
    try {
        return await new Promise<StreamInfo>((resolve, reject) => {
            const socket = net.createConnection(port, host, () => {
                socket.removeAllListeners('error');
                resolve({ reader: socket, writer: socket });
            });

            socket.once('error', (error) => {
                socket.destroy();
                reject(error);
            });
        });
    } catch (error) {
        if (attempt >= maxRetries) {
            outputChannel.error(`Failed to connect to language server at ${host}:${port} after ${maxRetries} attempts: ${error}`);
            throw error;
        }

        outputChannel.warn(`Connection attempt ${attempt}/${maxRetries} to ${host}:${port} failed, retrying in ${retryDelayMs}ms...`);
        await new Promise((r) => setTimeout(r, retryDelayMs));
        return connectWithRetry(host, port, maxRetries, retryDelayMs + (100 * (attempt + 1)), outputChannel, attempt + 1);
    }
}