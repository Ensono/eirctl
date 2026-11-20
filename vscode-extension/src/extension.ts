import * as vscode from 'vscode'
import { LanguageClient, LanguageClientOptions, State } from 'vscode-languageclient/node'
import { getServerOptions } from './init'

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
