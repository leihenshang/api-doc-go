// ESLint 扁平配置（T11 最小形态）：只做「推荐集」静态检查，**不引入 Prettier**（避免一次性重排既有文件）。
// 目标：拦住真问题（未定义变量、无效写法、Vue 模板误用），不纠缠纯格式。
import pluginVue from 'eslint-plugin-vue'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
    {
        // 构建产物与第三方目录不检查
        ignores: ['dist/**', 'dist-*/**', 'node_modules/**', 'public/**', '*.min.js'],
    },
    ...tseslint.configs.recommended,
    ...pluginVue.configs['flat/recommended'],
    {
        files: ['**/*.{js,mjs,cjs,ts,mts,tsx,vue}'],
        languageOptions: {
            ecmaVersion: 'latest',
            sourceType: 'module',
            globals: { ...globals.browser, ...globals.node },
        },
        rules: {
            // 允许 _ 前缀的未使用参数（沿用既有写法）
            '@typescript-eslint/no-unused-vars': [
                'warn',
                { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
            ],
            // 纯风格类规则关闭：本仓库不做统一格式化（未引入 Prettier）
            'vue/max-attributes-per-line': 'off',
            'vue/singleline-html-element-content-newline': 'off',
            'vue/html-indent': 'off',
            'vue/html-self-closing': 'off',
            'vue/attributes-order': 'off',
            'vue/first-attribute-linebreak': 'off',
            'vue/html-closing-bracket-newline': 'off',
            'vue/html-closing-bracket-spacing': 'off',
            'vue/html-quotes': 'off',
            'vue/attribute-hyphenation': 'off',
            'vue/v-on-event-hyphenation': 'off',
            'vue/no-v-html': 'off', // Markdown 预览/代码块等场景确实需要
            'vue/multiline-html-element-content-newline': 'off',
            'vue/mustache-interpolation-spacing': 'off',
        },
    },
    {
        // 类型声明文件（Vue SFC shim 等）允许 `{}` 与 `any`
        files: ['**/*.d.ts'],
        rules: {
            '@typescript-eslint/no-empty-object-type': 'off',
            '@typescript-eslint/no-explicit-any': 'off',
        },
    },
    {
        // .vue 的 <script lang="ts"> 交给 TS 解析器
        files: ['**/*.vue'],
        languageOptions: {
            parserOptions: { parser: tseslint.parser },
        },
    },
    {
        // 配置文件与脚本放宽
        files: ['*.config.{js,ts,mjs}', 'vite.config.ts'],
        rules: {
            '@typescript-eslint/no-explicit-any': 'off',
        },
    },
)
