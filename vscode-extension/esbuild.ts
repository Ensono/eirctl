import { BuildResult, Message, PluginBuild, context } from 'esbuild';
import { exit } from 'node:process';

/**
 * @type {import('esbuild').Plugin}
 */
const esbuildProblemMatcherPlugin = {
    name: 'esbuild-problem-matcher',

    setup(build: PluginBuild) {
        build.onStart(() => {
            console.log('[watch] build started');
        });
        build.onEnd((result: BuildResult) => {
            result.errors.forEach((message: Message) => {
                console.error(`✘ [ERROR] ${message.text}`);
                console.error(`    ${message?.location?.file}:${message?.location?.line}:${message?.location?.column}:`);
            });
            console.log('[watch] build finished');
        });
    },
};

(async () => {
    const production = process.argv.includes('--production');
    const watch = process.argv.includes('--watch');
    const ctx = await context({
        entryPoints: [
            'src/extension.ts'
        ],
        bundle: true,
        format: 'cjs',
        minify: production,
        sourcemap: !production,
        sourcesContent: false,
        platform: 'node',
        outfile: 'dist/extension.js',
        external: ['vscode'],
        logLevel: 'silent',
        plugins: [
            /* add to the end of plugins array */
            esbuildProblemMatcherPlugin,
        ],
    });
    if (watch) {
        await ctx.watch();
    } else {
        await ctx.rebuild();
        await ctx.dispose();
    }
})().catch((err) => {
    console.error(err);
    exit(1);
});
