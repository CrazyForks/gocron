/**
 * 离线图标加载器
 *
 * 用于在内网 / 国内网络环境下支持 Iconify 图标的离线加载。
 * 通过预加载图标集数据，避免运行时从 Iconify CDN (api.iconify.design) 获取图标
 * ——在无法访问 CDN 的环境下，图标会因请求失败而无法显示。
 *
 * 使用方式：
 * 1. 安装所需图标集：pnpm add -D @iconify-json/[icon-set-name]
 * 2. 在此文件中导入并注册图标集
 * 3. 在 main.ts 中以副作用方式引入本模块：import './utils/ui/iconify-loader'
 * 4. 在组件中使用：<ArtSvgIcon icon="ri:home-line" />
 *
 * @module utils/ui/iconify-loader
 * @author GoCronX Team
 */

import { addCollection } from '@iconify/vue'

// 导入离线图标数据
// ri（Remix Icon）：全站主图标集，含侧边栏、工具栏、GitHub 等图标
import riIcons from '@iconify-json/ri/icons.json'
// iconamoon：个别页面使用
import iconamoonIcons from '@iconify-json/iconamoon/icons.json'

// 注册离线图标集（注册整个集合，确保动态使用的图标名也能离线解析）
addCollection(riIcons)
addCollection(iconamoonIcons)
