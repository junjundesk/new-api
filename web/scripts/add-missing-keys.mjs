/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import fs from 'node:fs/promises'
import path from 'node:path'

// Adds missing performance-page keys to every locale without re-serializing the
// files, so unrelated formatting and ordering stay untouched. Keys are inserted
// next to their alphabetical neighbours; run "bun run i18n:sync" afterwards for
// the canonical ordering.
const LOCALES_DIR = path.resolve('src/i18n/locales')
const LOCALES = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi']

const NEW_KEYS = {
  Performance: {
    en: 'Performance',
    zh: '性能',
    'zh-TW': '效能',
    fr: 'Performances',
    ja: 'パフォーマンス',
    ru: 'Производительность',
    vi: 'Hiệu suất',
  },
  'Live availability and latency for the groups your account can use.': {
    en: 'Live availability and latency for the groups your account can use.',
    zh: '当前账号可用分组的实时可用率与延迟。',
    'zh-TW': '目前帳號可用分組的即時可用率與延遲。',
    fr: 'Disponibilité et latence en direct des groupes accessibles à votre compte.',
    ja: 'アカウントで利用可能なグループのリアルタイム可用性とレイテンシ。',
    ru: 'Доступность и задержка в реальном времени для групп, доступных вашему аккаунту.',
    vi: 'Tình trạng khả dụng và độ trễ trực tiếp của các nhóm mà tài khoản của bạn có thể sử dụng.',
  },
  'Updated {{time}}': {
    en: 'Updated {{time}}',
    zh: '更新于 {{time}}',
    'zh-TW': '更新於 {{time}}',
    fr: 'Mis à jour à {{time}}',
    ja: '{{time}} 更新',
    ru: 'Обновлено в {{time}}',
    vi: 'Cập nhật lúc {{time}}',
  },
  'Auto-refreshes every minute': {
    en: 'Auto-refreshes every minute',
    zh: '每分钟自动刷新',
    'zh-TW': '每分鐘自動重新整理',
    fr: 'Actualisation automatique chaque minute',
    ja: '毎分自動更新',
    ru: 'Автообновление каждую минуту',
    vi: 'Tự động làm mới mỗi phút',
  },
  'Time range': {
    en: 'Time range',
    zh: '时间范围',
    'zh-TW': '時間範圍',
    fr: 'Période',
    ja: '期間',
    ru: 'Период',
    vi: 'Khoảng thời gian',
  },
  'Last 24 hours': {
    en: 'Last 24 hours',
    zh: '最近 24 小时',
    'zh-TW': '最近 24 小時',
    fr: 'Dernières 24 heures',
    ja: '過去24時間',
    ru: 'Последние 24 часа',
    vi: '24 giờ qua',
  },
  'Last 7 days': {
    en: 'Last 7 days',
    zh: '最近 7 天',
    'zh-TW': '最近 7 天',
    fr: '7 derniers jours',
    ja: '過去7日間',
    ru: 'Последние 7 дней',
    vi: '7 ngày qua',
  },
  Refresh: {
    en: 'Refresh',
    zh: '刷新',
    'zh-TW': '重新整理',
    fr: 'Actualiser',
    ja: '更新',
    ru: 'Обновить',
    vi: 'Làm mới',
  },
  'Sort groups': {
    en: 'Sort groups',
    zh: '排序分组',
    'zh-TW': '排序群組',
    fr: 'Trier les groupes',
    ja: 'グループを並べ替え',
    ru: 'Сортировать группы',
    vi: 'Sắp xếp nhóm',
  },
  'Custom group order': {
    en: 'Custom group order',
    zh: '自定义分组顺序',
    'zh-TW': '自訂群組順序',
    fr: 'Ordre personnalisé des groupes',
    ja: 'グループのカスタム順序',
    ru: 'Свой порядок групп',
    vi: 'Thứ tự nhóm tùy chỉnh',
  },
  'Request volume': {
    en: 'Request volume',
    zh: '请求量',
    'zh-TW': '請求量',
    fr: 'Volume de requêtes',
    ja: 'リクエスト量',
    ru: 'Объём запросов',
    vi: 'Lưu lượng yêu cầu',
  },
  'Observed multiplier: high to low': {
    en: 'Observed multiplier: high to low',
    zh: '实测倍率：从高到低',
    'zh-TW': '實測倍率：由高到低',
    fr: 'Multiplicateur observé : décroissant',
    ja: '実測倍率：高い順',
    ru: 'Наблюдаемый множитель: по убыванию',
    vi: 'Hệ số thực tế: cao đến thấp',
  },
  'Observed multiplier: low to high': {
    en: 'Observed multiplier: low to high',
    zh: '实测倍率：从低到高',
    'zh-TW': '實測倍率：由低到高',
    fr: 'Multiplicateur observé : croissant',
    ja: '実測倍率：低い順',
    ru: 'Наблюдаемый множитель: по возрастанию',
    vi: 'Hệ số thực tế: thấp đến cao',
  },
  'Effective multiplier: high to low': {
    en: 'Effective multiplier: high to low',
    zh: '实际倍率：从高到低',
    'zh-TW': '實際倍率：由高到低',
    fr: 'Multiplicateur effectif : décroissant',
    ja: '実効倍率：高い順',
    ru: 'Эффективный множитель: по убыванию',
    vi: 'Hệ số thực tế: cao đến thấp',
  },
  'Effective multiplier: low to high': {
    en: 'Effective multiplier: low to high',
    zh: '实际倍率：从低到高',
    'zh-TW': '實際倍率：由低到高',
    fr: 'Multiplicateur effectif : croissant',
    ja: '実効倍率：低い順',
    ru: 'Эффективный множитель: по возрастанию',
    vi: 'Hệ số thực tế: thấp đến cao',
  },
  'Group name': {
    en: 'Group name',
    zh: '分组名称',
    'zh-TW': '分組名稱',
    fr: 'Nom du groupe',
    ja: 'グループ名',
    ru: 'Имя группы',
    vi: 'Tên nhóm',
  },
  'Coding cache: high to low': {
    en: 'Coding cache: high to low',
    zh: 'Coding 缓存：从高到低',
    'zh-TW': 'Coding 快取：由高到低',
    fr: 'Cache Coding : décroissant',
    ja: 'Coding キャッシュ：高い順',
    ru: 'Кэш Coding: по убыванию',
    vi: 'Bộ nhớ đệm Coding: cao đến thấp',
  },
  'Success rate: high to low': {
    en: 'Success rate: high to low',
    zh: '成功率：从高到低',
    'zh-TW': '成功率：由高到低',
    fr: 'Taux de réussite : décroissant',
    ja: '成功率：高い順',
    ru: 'Успешность: по убыванию',
    vi: 'Tỷ lệ thành công: cao đến thấp',
  },
  'Latency: low to high': {
    en: 'Latency: low to high',
    zh: '延迟：从低到高',
    'zh-TW': '延遲：由低到高',
    fr: 'Latence : croissante',
    ja: 'レイテンシ：低い順',
    ru: 'Задержка: по возрастанию',
    vi: 'Độ trễ: thấp đến cao',
  },
  'Throughput: high to low': {
    en: 'Throughput: high to low',
    zh: '吞吐量：从高到低',
    'zh-TW': '吞吐量：由高到低',
    fr: 'Débit : décroissant',
    ja: 'スループット：高い順',
    ru: 'Пропускная способность: по убыванию',
    vi: 'Thông lượng: cao đến thấp',
  },
  'Filter groups by status': {
    en: 'Filter groups by status',
    zh: '按状态筛选分组',
    'zh-TW': '按狀態篩選分組',
    fr: 'Filtrer les groupes par statut',
    ja: 'ステータスでグループを絞り込む',
    ru: 'Фильтровать группы по статусу',
    vi: 'Lọc nhóm theo trạng thái',
  },
  Operational: {
    en: 'Operational',
    zh: '运行中',
    'zh-TW': '運作正常',
    fr: 'Opérationnel',
    ja: '稼働中',
    ru: 'Работает',
    vi: 'Đang vận hành',
  },
  Degraded: {
    en: 'Degraded',
    zh: '波动',
    'zh-TW': '波動',
    fr: 'Dégradé',
    ja: '低下',
    ru: 'Деградация',
    vi: 'Suy giảm',
  },
  Unavailable: {
    en: 'Unavailable',
    zh: '异常',
    'zh-TW': '異常',
    fr: 'Indisponible',
    ja: '利用不可',
    ru: 'Недоступно',
    vi: 'Không khả dụng',
  },
  'No data': {
    en: 'No data',
    zh: '暂无数据',
    'zh-TW': '暫無數據',
    fr: 'Aucune donnée',
    ja: 'データがありません',
    ru: 'Нет данных',
    vi: 'Không có dữ liệu',
  },
  '{{shown}} of {{count}} groups': {
    en: '{{shown}} of {{count}} groups',
    zh: '{{shown}}/{{count}} 个分组',
    'zh-TW': '{{shown}}/{{count}} 個分組',
    fr: '{{shown}} groupes sur {{count}}',
    ja: '{{count}} グループ中 {{shown}} 件',
    ru: '{{shown}} из {{count}} групп',
    vi: '{{shown}} trong số {{count}} nhóm',
  },
  '{{count}} groups': {
    en: '{{count}} groups',
    zh: '{{count}} 个分组',
    'zh-TW': '{{count}} 個分組',
    fr: '{{count}} groupes',
    ja: '{{count}} 個のグループ',
    ru: 'Групп: {{count}}',
    vi: '{{count}} nhóm',
  },
  'Use Move up and Move down to save your group order on this device. Filters keep hidden groups in place.':
    {
      en: 'Use Move up and Move down to save your group order on this device. Filters keep hidden groups in place.',
      zh: '使用上移、下移调整顺序，并保存在此设备。筛选时隐藏的分组保持原位置。',
      'zh-TW':
        '使用上移、下移調整順序，並儲存在此裝置。篩選時隱藏的群組保持原位置。',
      fr: 'Utilisez Monter et Descendre pour enregistrer l’ordre sur cet appareil. Les groupes masqués par un filtre restent en place.',
      ja: '上へ・下へ移動で順序を調整し、この端末に保存します。フィルターで非表示のグループは元の位置を保持します。',
      ru: 'Кнопки перемещения сохраняют порядок на этом устройстве. Скрытые фильтром группы остаются на своих местах.',
      vi: 'Dùng Di chuyển lên và Di chuyển xuống để lưu thứ tự trên thiết bị này. Các nhóm bị bộ lọc ẩn giữ nguyên vị trí.',
    },
  'Move up': {
    en: 'Move up',
    zh: '上移',
    'zh-TW': 'Move up',
    fr: 'Déplacer vers le haut',
    ja: '上へ移動',
    ru: 'Переместить вверх',
    vi: 'Di chuyển lên',
  },
  'Move down': {
    en: 'Move down',
    zh: '下移',
    'zh-TW': 'Move down',
    fr: 'Déplacer vers le bas',
    ja: '下へ移動',
    ru: 'Переместить вниз',
    vi: 'Di chuyển xuống',
  },
  'Move {{group}} up': {
    en: 'Move {{group}} up',
    zh: '将 {{group}} 上移',
    'zh-TW': '將 {{group}} 上移',
    fr: 'Monter {{group}}',
    ja: '{{group}} を上へ移動',
    ru: 'Переместить {{group}} вверх',
    vi: 'Di chuyển {{group}} lên',
  },
  'Move {{group}} down': {
    en: 'Move {{group}} down',
    zh: '将 {{group}} 下移',
    'zh-TW': '將 {{group}} 下移',
    fr: 'Descendre {{group}}',
    ja: '{{group}} を下へ移動',
    ru: 'Переместить {{group}} вниз',
    vi: 'Di chuyển {{group}} xuống',
  },
  'Group ratio': {
    en: 'Group ratio',
    zh: '分组倍率',
    'zh-TW': '分組倍率',
    fr: 'Ratio du groupe',
    ja: 'グループ倍率',
    ru: 'Коэффициент группы',
    vi: 'Hệ số nhóm',
  },
  'User Group': {
    en: 'User Group',
    zh: '用户分组',
    'zh-TW': '用戶分組',
    fr: "Groupe d'utilisateurs",
    ja: 'ユーザーグループ',
    ru: 'Группа пользователей',
    vi: 'Nhóm người dùng',
  },
  Auto: {
    en: 'Auto',
    zh: '自动',
    'zh-TW': '自動',
    fr: 'Auto',
    ja: '自動',
    ru: 'Авто',
    vi: 'Tự động',
  },
  'Cache hit': {
    en: 'Cache hit',
    zh: '缓存命中',
    'zh-TW': '快取命中',
    fr: 'Cache utilisé',
    ja: 'キャッシュヒット',
    ru: 'Попадание в кэш',
    vi: 'Trúng bộ nhớ đệm',
  },
  'Coding cache': {
    en: 'Coding cache',
    zh: 'Coding 缓存',
    'zh-TW': 'Coding 快取',
    fr: 'Cache Coding',
    ja: 'Coding キャッシュ',
    ru: 'Кэш Coding',
    vi: 'Bộ nhớ đệm Coding',
  },
  'Observed input multiplier': {
    en: 'Observed input multiplier',
    zh: '实测输入倍率',
    'zh-TW': '實測輸入倍率',
    fr: 'Multiplicateur d’entrée observé',
    ja: '実測入力倍率',
    ru: 'Наблюдаемый множитель входа',
    vi: 'Hệ số đầu vào thực tế',
  },
  'Reference-adjusted multiplier': {
    en: 'Reference-adjusted multiplier',
    zh: '参考调整倍率',
    'zh-TW': '參考調整倍率',
    fr: 'Multiplicateur ajusté à la référence',
    ja: '参照調整倍率',
    ru: 'Множитель с поправкой на эталон',
    vi: 'Hệ số điều chỉnh theo tham chiếu',
  },
  'Pricing coverage is insufficient': {
    en: 'Pricing coverage is insufficient',
    zh: '价格覆盖不足',
    'zh-TW': '價格覆蓋不足',
    fr: 'Couverture tarifaire insuffisante',
    ja: '料金カバレッジが不足しています',
    ru: 'Недостаточное покрытие ценами',
    vi: 'Độ phủ giá chưa đủ',
  },
  'Cost coverage: {{coverage}}%': {
    en: 'Cost coverage: {{coverage}}%',
    zh: '成本覆盖：{{coverage}}%',
    'zh-TW': '成本涵蓋：{{coverage}}%',
    fr: 'Couverture des coûts : {{coverage}} %',
    ja: '費用の観測率：{{coverage}}%',
    ru: 'Покрытие стоимости: {{coverage}}%',
    vi: 'Mức bao phủ chi phí: {{coverage}}%',
  },
  Latency: {
    en: 'Latency',
    zh: '延迟',
    'zh-TW': '延遲',
    fr: 'Latence',
    ja: 'レイテンシ',
    ru: 'Задержка',
    vi: 'Độ trễ',
  },
  Availability: {
    en: 'Availability',
    zh: '可用性',
    'zh-TW': '可用性',
    fr: 'Disponibilité',
    ja: '可用性',
    ru: 'Доступность',
    vi: 'Khả dụng',
  },
  '{{success}}/{{total}} requests succeeded': {
    en: '{{success}}/{{total}} requests succeeded',
    zh: '{{success}}/{{total}} 次请求成功',
    'zh-TW': '{{success}}/{{total}} 次請求成功',
    fr: '{{success}}/{{total}} requêtes réussies',
    ja: '{{success}}/{{total}} 件のリクエストが成功',
    ru: 'Успешно {{success}}/{{total}} запросов',
    vi: '{{success}}/{{total}} yêu cầu thành công',
  },
  'No requests in this window': {
    en: 'No requests in this window',
    zh: '该时间窗内无请求',
    'zh-TW': '該時間窗內無請求',
    fr: 'Aucune requête sur cette période',
    ja: 'この期間のリクエストはありません',
    ru: 'Нет запросов за этот период',
    vi: 'Không có yêu cầu trong khoảng thời gian này',
  },
  '{{count}} models with traffic': {
    en: '{{count}} models with traffic',
    zh: '{{count}} 个模型有流量',
    'zh-TW': '{{count}} 個模型有流量',
    fr: '{{count}} modèles avec du trafic',
    ja: 'トラフィックのあるモデル: {{count}}',
    ru: 'Моделей с трафиком: {{count}}',
    vi: '{{count}} mô hình có lưu lượng',
  },
  'View models': {
    en: 'View models',
    zh: '查看模型',
    'zh-TW': '檢視模型',
    fr: 'Voir les modèles',
    ja: 'モデルを表示',
    ru: 'Показать модели',
    vi: 'Xem mô hình',
  },
  Past: {
    en: 'Past',
    zh: '过去',
    'zh-TW': '過去',
    fr: 'Passé',
    ja: '過去',
    ru: 'Ранее',
    vi: 'Trước',
  },
  Now: {
    en: 'Now',
    zh: '现在',
    'zh-TW': '現在',
    fr: 'Maintenant',
    ja: '現在',
    ru: 'Сейчас',
    vi: 'Bây giờ',
  },
  '{{count}} requests': {
    en: '{{count}} requests',
    zh: '{{count}} 次请求',
    'zh-TW': '{{count}} 次請求',
    fr: '{{count}} requêtes',
    ja: '{{count}} 件のリクエスト',
    ru: 'Запросов: {{count}}',
    vi: '{{count}} yêu cầu',
  },
  'Cache metrics explained': {
    en: 'Cache metrics explained',
    zh: '缓存统计说明',
    'zh-TW': '快取統計說明',
    fr: 'Calcul du cache',
    ja: 'キャッシュ統計の説明',
    ru: 'Расчёт кэша',
    vi: 'Giải thích thống kê bộ nhớ đệm',
  },
  'Cache hit is cached input tokens divided by total input tokens, summed across successful requests in this group. Output tokens are excluded.':
    {
      en: 'Cache hit is cached input tokens divided by total input tokens, summed across successful requests in this group. Output tokens are excluded.',
      zh: '缓存命中 = 缓存读取 token 总数 ÷ 总输入 token 数，按本分组的成功调用汇总，不包含输出 token。',
      'zh-TW':
        '快取命中 = 快取讀取 token 總數 ÷ 總輸入 token 數，按本群組的成功呼叫彙總，不包含輸出 token。',
      fr: 'Le taux de cache est le nombre de tokens lus en cache divisé par le total des tokens d’entrée, cumulés sur les requêtes réussies du groupe. Les tokens de sortie sont exclus.',
      ja: 'キャッシュヒット率は、グループ内で成功したリクエストのキャッシュ読み取りトークン合計を入力トークン合計で割った値です。出力トークンは含みません。',
      ru: 'Доля кэша — сумма прочитанных из кэша токенов, делённая на сумму входных токенов успешных запросов группы. Выходные токены не учитываются.',
      vi: 'Tỷ lệ trúng bộ nhớ đệm bằng tổng token đọc từ bộ nhớ đệm chia cho tổng token đầu vào của các yêu cầu thành công trong nhóm. Không bao gồm token đầu ra.',
    },
  'Coding cache uses the same calculation, only for Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH), and Open Design clients.':
    {
      en: 'Coding cache uses the same calculation, only for Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH), and Open Design clients.',
      zh: 'Coding 缓存采用相同公式，仅统计 Codex、Claude Code、Pi、OpenCode、OMP、ZCode、DeepSeek Harness (DSH) 和 Open Design 客户端。',
      'zh-TW':
        'Coding 快取採用相同公式，僅統計 Codex、Claude Code、Pi、OpenCode、OMP、ZCode、DeepSeek Harness (DSH) 和 Open Design 用戶端。',
      fr: 'Le cache Coding utilise le même calcul, uniquement pour les clients Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH) et Open Design.',
      ja: 'Coding キャッシュは同じ計算式で、Codex、Claude Code、Pi、OpenCode、OMP、ZCode、DeepSeek Harness (DSH)、Open Design のクライアントのみを集計します。',
      ru: 'Кэш Coding рассчитывается так же, но только для клиентов Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH) и Open Design.',
      vi: 'Bộ nhớ đệm Coding dùng cùng công thức, chỉ tính các ứng dụng Codex, Claude Code, Pi, OpenCode, OMP, ZCode, DeepSeek Harness (DSH) và Open Design.',
    },
  'Observed multiplier = input cost ÷ the 0% cache reference cost. Reference-adjusted multiplier uses your selected cache scenario instead. Both keep cache-write costs fixed and require 90% input cost coverage.':
    {
      en: 'Observed multiplier = input cost ÷ the 0% cache reference cost. Reference-adjusted multiplier uses your selected cache scenario instead. Both keep cache-write costs fixed and require 90% input cost coverage.',
      zh: '观测倍率 = 输入成本 ÷ 缓存 0% 的参考成本；参考调整倍率使用你选择的缓存场景作为分母。两者均保持写入费用不变，输入成本覆盖达到 90% 才展示。',
      'zh-TW':
        '觀測倍率 = 輸入成本 ÷ 快取 0% 的參考成本；參考調整倍率使用你選擇的快取情境作為分母。兩者均保持寫入費用不變，輸入成本涵蓋達到 90% 才顯示。',
      fr: 'Multiplicateur observé = coût d’entrée ÷ coût de référence avec 0 % de cache. Le multiplicateur ajusté utilise le scénario choisi. Les deux conservent le coût des écritures et exigent 90 % de couverture.',
      ja: '観測倍率は入力費用をキャッシュ率 0% の基準費用で割った値です。基準調整倍率では選択したキャッシュ率を使います。両方とも書き込み費用は固定で、入力費用の観測率 90% 以上が必要です。',
      ru: 'Наблюдаемый множитель = стоимость ввода ÷ эталонная стоимость при кеше 0%. Скорректированный множитель использует выбранный сценарий. Стоимость записи фиксирована; требуется 90% покрытия стоимости ввода.',
      vi: 'Hệ số quan sát = chi phí đầu vào ÷ chi phí tham chiếu bộ nhớ đệm 0%. Hệ số điều chỉnh dùng kịch bản bạn chọn. Cả hai giữ nguyên chi phí ghi và yêu cầu mức bao phủ chi phí đầu vào 90%.',
    },
  'Only valid upstream usage is included. No data is shown as —; a measured zero is 0%. Statistics cover all users in the group.':
    {
      en: 'Only valid upstream usage is included. No data is shown as —; a measured zero is 0%. Statistics cover all users in the group.',
      zh: '仅纳入有效的上游用量。无数据时显示 —，真实零命中显示 0%。统计覆盖本分组的所有用户。',
      'zh-TW':
        '僅納入有效的上游用量。無資料時顯示 —，實際零命中顯示 0%。統計涵蓋本群組的所有使用者。',
      fr: 'Seuls les usages amont valides sont inclus. L’absence de données est indiquée par — ; un zéro mesuré par 0 %. Les statistiques couvrent tous les utilisateurs du groupe.',
      ja: '有効な上流使用量のみを含みます。データなしは —、実測でヒットなしは 0% と表示します。グループ内の全ユーザーが対象です。',
      ru: 'Учитываются только корректные данные провайдера. Нет данных — знак —; измеренное отсутствие попаданий — 0%. Статистика охватывает всех пользователей группы.',
      vi: 'Chỉ tính dữ liệu sử dụng hợp lệ từ nhà cung cấp. Không có dữ liệu hiển thị —; đo được không trúng hiển thị 0%. Thống kê bao gồm mọi người dùng trong nhóm.',
    },
  'Reference cache rate': {
    en: 'Reference cache rate',
    zh: '参考缓存率',
    'zh-TW': '參考快取率',
    fr: 'Taux de cache de référence',
    ja: '基準キャッシュ率',
    ru: 'Эталонная доля кэша',
    vi: 'Tỷ lệ bộ nhớ đệm tham chiếu',
  },
  'Reference cache: {{rate}}%': {
    en: 'Reference cache: {{rate}}%',
    zh: '参考缓存率：{{rate}}%',
    'zh-TW': '參考快取率：{{rate}}%',
    fr: 'Cache de référence : {{rate}} %',
    ja: '基準キャッシュ率：{{rate}}%',
    ru: 'Эталонный кэш: {{rate}}%',
    vi: 'Bộ nhớ đệm tham chiếu: {{rate}}%',
  },
  'Reference cache rate (%)': {
    en: 'Reference cache rate (%)',
    zh: '参考缓存率（%）',
    'zh-TW': '參考快取率（%）',
    fr: 'Taux de cache de référence (%)',
    ja: '基準キャッシュ率（%）',
    ru: 'Эталонная доля кэша (%)',
    vi: 'Tỷ lệ bộ nhớ đệm tham chiếu (%)',
  },
  '90% is an adjustable multi-turn coding scenario, not a guaranteed official cache rate. OpenAI documents examples above 90%, with results depending on the workload.':
    {
      en: '90% is an adjustable multi-turn coding scenario, not a guaranteed official cache rate. OpenAI documents examples above 90%, with results depending on the workload.',
      zh: '90% 是可调整的多轮 Coding 参考场景，不是官方保证的缓存率。OpenAI 文档列举了超过 90% 的案例，实际效果取决于工作负载。',
      'zh-TW':
        '90% 是可調整的多輪 Coding 參考情境，並非官方保證的快取率。OpenAI 文件列舉了超過 90% 的案例，實際效果取決於工作負載。',
      fr: '90 % est un scénario ajustable de codage à plusieurs tours, pas un taux garanti par le fournisseur. OpenAI présente des exemples dépassant 90 %, selon la charge de travail.',
      ja: '90% は調整可能な複数ターン Coding の参考値で、公式に保証されたキャッシュ率ではありません。OpenAI は 90% を超える事例を示していますが、結果はワークロードに依存します。',
      ru: '90% — настраиваемый сценарий многошаговой работы Coding, а не гарантированная провайдером доля кэша. OpenAI приводит примеры выше 90%, но результат зависит от нагрузки.',
      vi: '90% là kịch bản Coding nhiều lượt có thể điều chỉnh, không phải tỷ lệ được nhà cung cấp bảo đảm. OpenAI đưa ra ví dụ trên 90%, nhưng kết quả phụ thuộc vào khối lượng công việc.',
    },
  'Changing this reference only changes the estimate, not your billing.': {
    en: 'Changing this reference only changes the estimate, not your billing.',
    zh: '调整参考值只影响估算结果，不会改变计费。',
    'zh-TW': '調整參考值只影響估算結果，不會改變計費。',
    fr: 'Modifier cette référence change uniquement l’estimation, pas votre facturation.',
    ja: '基準値の変更は推定結果にのみ影響し、請求は変わりません。',
    ru: 'Изменение эталона влияет только на оценку, а не на оплату.',
    vi: 'Thay đổi giá trị tham chiếu chỉ ảnh hưởng đến ước tính, không thay đổi phí của bạn.',
  },
  'The reference changes cache reads for non-write input. Actual cache-write costs stay fixed; Coding cache percentages include writes in total input.':
    {
      en: 'The reference changes cache reads for non-write input. Actual cache-write costs stay fixed; Coding cache percentages include writes in total input.',
      zh: '参考缓存率仅调整非写入输入的缓存读取；缓存写入费用保持不变。Coding 缓存率的输入总量包含写入。',
      'zh-TW':
        '參考快取率僅調整非寫入輸入的快取讀取；快取寫入費用保持不變。Coding 快取率的輸入總量包含寫入。',
      fr: 'La référence modifie les lectures du cache pour les entrées hors écriture. Le coût des écritures reste fixe ; le taux Coding inclut les écritures dans le total des entrées.',
      ja: '基準値は書き込み以外の入力のキャッシュ読み取り率を変更します。書き込み費用は固定です。Coding キャッシュ率の入力総量には書き込みも含まれます。',
      ru: 'Эталон меняет долю чтения кеша только для входных токенов без записи. Стоимость записи остаётся фиксированной; доля кеша Coding учитывает записи в общем объёме ввода.',
      vi: 'Mức tham chiếu chỉ thay đổi tỷ lệ đọc bộ nhớ đệm của đầu vào không ghi. Chi phí ghi giữ nguyên; tỷ lệ Coding tính cả phần ghi trong tổng đầu vào.',
    },
  'Input price estimate': {
    en: 'Input price estimate',
    zh: '输入价格估算',
    'zh-TW': '輸入價格估算',
    fr: 'Estimation du prix d’entrée',
    ja: '入力価格の推定',
    ru: 'Оценка стоимости ввода',
    vi: 'Ước tính giá đầu vào',
  },
  'USD per 1M input tokens': {
    en: 'USD per 1M input tokens',
    zh: '美元 / 百万输入 token',
    'zh-TW': '美元 / 百萬輸入 token',
    fr: 'USD par million de tokens d’entrée',
    ja: '入力100万トークンあたりの米ドル',
    ru: 'USD за 1 млн входных токенов',
    vi: 'USD trên 1 triệu token đầu vào',
  },
  'Input price at 0% reference cache': {
    en: 'Input price at 0% reference cache',
    zh: '参考缓存 0% 时的输入价',
    'zh-TW': '參考快取 0% 時的輸入價',
    fr: 'Prix d’entrée avec 0 % de cache de référence',
    ja: '基準キャッシュ率 0% の入力価格',
    ru: 'Цена ввода при эталонном кеше 0%',
    vi: 'Giá đầu vào khi bộ nhớ đệm tham chiếu là 0%',
  },
  'Input price at 100% reference cache': {
    en: 'Input price at 100% reference cache',
    zh: '参考缓存 100% 时的输入价',
    'zh-TW': '參考快取 100% 時的輸入價',
    fr: 'Prix d’entrée avec 100 % de cache de référence',
    ja: '基準キャッシュ率 100% の入力価格',
    ru: 'Цена ввода при эталонном кеше 100%',
    vi: 'Giá đầu vào khi bộ nhớ đệm tham chiếu là 100%',
  },
  'Observed input price': {
    en: 'Observed input price',
    zh: '观测输入单价',
    'zh-TW': '觀測輸入單價',
    fr: 'Prix d’entrée observé',
    ja: '観測された入力価格',
    ru: 'Наблюдаемая цена ввода',
    vi: 'Giá đầu vào quan sát được',
  },
  'Reference input price': {
    en: 'Reference input price',
    zh: '参考输入单价',
    'zh-TW': '參考輸入單價',
    fr: 'Prix d’entrée de référence',
    ja: '基準入力単価',
    ru: 'Эталонная цена ввода',
    vi: 'Giá đầu vào tham chiếu',
  },
  'Reference cache {{rate}}% · all Coding cache {{observed}}% · input cost coverage {{coverage}}%':
    {
      en: 'Reference cache {{rate}}% · all Coding cache {{observed}}% · input cost coverage {{coverage}}%',
      zh: '参考缓存 {{rate}}% · 全部 Coding 缓存 {{observed}}% · 输入成本覆盖 {{coverage}}%',
      'zh-TW':
        '參考快取 {{rate}}% · 全部 Coding 快取 {{observed}}% · 輸入成本涵蓋 {{coverage}}%',
      fr: 'Cache de référence {{rate}} % · cache Coding global {{observed}} % · couverture des coûts d’entrée {{coverage}} %',
      ja: '基準キャッシュ {{rate}}% · Coding 全体のキャッシュ {{observed}}% · 入力費用の観測率 {{coverage}}%',
      ru: 'Эталонный кеш {{rate}}% · весь кеш Coding {{observed}}% · покрытие стоимости ввода {{coverage}}%',
      vi: 'Bộ nhớ đệm tham chiếu {{rate}}% · bộ nhớ đệm Coding tổng thể {{observed}}% · mức bao phủ chi phí đầu vào {{coverage}}%',
    },
  'Prices describe the observed subset. Multipliers require 90% input cost coverage for the group or model being shown.':
    {
      en: 'Prices describe the observed subset. Multipliers require 90% input cost coverage for the group or model being shown.',
      zh: '单价仅描述已观测的样本。分组或模型的输入成本覆盖达到 90% 后，才展示对应倍率。',
      'zh-TW':
        '單價僅描述已觀測的樣本。群組或模型的輸入成本涵蓋達到 90% 後，才顯示對應倍率。',
      fr: 'Les prix décrivent uniquement les données observées. L’affichage d’un multiplicateur exige 90 % de couverture des coûts d’entrée du groupe ou du modèle concerné.',
      ja: '価格は観測済みのサンプルのみを表します。倍率は対象のグループまたはモデルの入力費用の観測率が 90% 以上で表示されます。',
      ru: 'Цены относятся только к наблюдаемой выборке. Для показа множителя требуется 90% покрытия стоимости ввода соответствующей группы или модели.',
      vi: 'Giá chỉ mô tả tập dữ liệu đã quan sát. Hệ số chỉ hiển thị khi mức bao phủ chi phí đầu vào của nhóm hoặc mô hình đạt 90%.',
    },
  'Observed input cost uses each request’s settlement prices and multiplier. Reference cost uses the same pricing rules before the group multiplier; these are site prices, not verified official prices.':
    {
      en: 'Observed input cost uses each request’s settlement prices and multiplier. Reference cost uses the same pricing rules before the group multiplier; these are site prices, not verified official prices.',
      zh: '观测成本采用每次请求结算时的价格与倍率。参考成本沿用同一计价规则、不含分组倍率；基准来自站点配置，未经官网价格核验。',
      'zh-TW':
        '觀測成本採用每次請求結算時的價格與倍率。參考成本沿用相同計價規則、不含群組倍率；基準來自站點設定，未經官網價格核驗。',
      fr: 'Le coût observé utilise les prix et multiplicateurs appliqués à chaque requête. La référence conserve ces règles avant le multiplicateur du groupe ; il s’agit des prix du site, sans vérification des tarifs officiels.',
      ja: '観測費用には各リクエストの精算時の価格と倍率を使います。基準費用は同じ料金規則からグループ倍率を除いたものです。サイト設定の価格であり、公式価格との照合は行っていません。',
      ru: 'Наблюдаемая стоимость использует цены и множитель при расчёте каждого запроса. Эталон применяет те же правила без множителя группы; это цены сайта, не сверенные с официальными.',
      vi: 'Chi phí quan sát dùng giá và hệ số khi quyết toán từng yêu cầu. Chi phí tham chiếu dùng cùng quy tắc trước hệ số nhóm; đây là giá cấu hình của trang, chưa đối chiếu giá chính thức.',
    },
  'Input costs include regular input, cache reads, and cache writes. Output, tools, and recharge discounts are excluded. Historical usage without cost observations and unsupported pricing reduce coverage.':
    {
      en: 'Input costs include regular input, cache reads, and cache writes. Output, tools, and recharge discounts are excluded. Historical usage without cost observations and unsupported pricing reduce coverage.',
      zh: '输入成本包含普通输入、缓存读取和缓存写入，不含输出、工具费用与充值优惠。未采集成本的历史用量及不支持的计价规则会降低覆盖率。',
      'zh-TW':
        '輸入成本包含一般輸入、快取讀取和快取寫入，不含輸出、工具費用與儲值優惠。未採集成本的歷史用量及不支援的計價規則會降低涵蓋率。',
      fr: 'Les coûts d’entrée incluent l’entrée normale, les lectures et écritures du cache. Sorties, outils et remises de recharge sont exclus. L’historique sans coûts observés et les tarifs non pris en charge réduisent la couverture.',
      ja: '入力費用には通常入力、キャッシュ読み取り、書き込みを含みます。出力、ツール、チャージ割引は含みません。費用未収集の過去の利用や未対応の料金規則は観測率を下げます。',
      ru: 'Стоимость ввода включает обычный ввод, чтение и запись кеша. Вывод, инструменты и скидки на пополнение исключены. История без наблюдений стоимости и неподдерживаемые тарифы снижают покрытие.',
      vi: 'Chi phí đầu vào gồm đầu vào thường, đọc và ghi bộ nhớ đệm. Không tính đầu ra, công cụ và ưu đãi nạp tiền. Dữ liệu cũ chưa thu thập chi phí và quy tắc giá chưa hỗ trợ làm giảm mức bao phủ.',
    },
  'Costs are calculated before quota rounding. Nonlinear, cache-dependent, media, and per-request pricing may remain unobserved.':
    {
      en: 'Costs are calculated before quota rounding. Nonlinear, cache-dependent, media, and per-request pricing may remain unobserved.',
      zh: '成本在整数额度舍入前计算。非线性、依赖缓存条件、媒体及按次计费等规则可能没有成本观测。',
      'zh-TW':
        '成本在整數額度捨入前計算。非線性、依賴快取條件、媒體及按次計費等規則可能沒有成本觀測。',
      fr: 'Les coûts sont calculés avant l’arrondi du quota. Les tarifs non linéaires, dépendants du cache, multimédias ou par requête peuvent rester sans observation.',
      ja: '費用はクォータの整数丸め前に計算します。非線形、キャッシュ依存、メディア、リクエスト単位の料金は観測対象外となる場合があります。',
      ru: 'Стоимость рассчитывается до округления квоты. Нелинейные, зависящие от кеша, медийные и позапросные тарифы могут оставаться без наблюдений.',
      vi: 'Chi phí được tính trước khi làm tròn hạn ngạch. Giá phi tuyến, phụ thuộc bộ nhớ đệm, đa phương tiện và theo yêu cầu có thể chưa được quan sát.',
    },
  'No valid Coding input was observed in this window.': {
    en: 'No valid Coding input was observed in this window.',
    zh: '此时间窗口内没有有效的 Coding 输入观测。',
    'zh-TW': '此時間範圍內沒有有效的 Coding 輸入觀測。',
    fr: 'Aucune entrée Coding valide n’a été observée sur cette période.',
    ja: 'この期間には有効な Coding 入力の観測がありません。',
    ru: 'За этот период не наблюдалось корректного ввода Coding.',
    vi: 'Không có dữ liệu đầu vào Coding hợp lệ trong khoảng thời gian này.',
  },
  'Model breakdown': {
    en: 'Model breakdown',
    zh: '模型明细',
    'zh-TW': '模型明細',
    fr: 'Détail par modèle',
    ja: 'モデル別内訳',
    ru: 'Разбивка по моделям',
    vi: 'Chi tiết theo mô hình',
  },
  'Per-model availability within this group': {
    en: 'Per-model availability within this group',
    zh: '该分组内按模型的可用率',
    'zh-TW': '該分組內按模型的可用率',
    fr: 'Disponibilité par modèle dans ce groupe',
    ja: 'このグループ内のモデル別可用性',
    ru: 'Доступность по моделям в этой группе',
    vi: 'Khả dụng theo từng mô hình trong nhóm này',
  },
  Model: {
    en: 'Model',
    zh: '模型',
    'zh-TW': '模型',
    fr: 'Modèle',
    ja: 'モデル',
    ru: 'Модель',
    vi: 'Mô hình',
  },
  Requests: {
    en: 'Requests',
    zh: '请求数',
    'zh-TW': '請求數',
    fr: 'Requêtes',
    ja: 'リクエスト',
    ru: 'Запросы',
    vi: 'Yêu cầu',
  },
  'Success rate': {
    en: 'Success rate',
    zh: '成功率',
    'zh-TW': '成功率',
    fr: 'Taux de réussite',
    ja: '成功率',
    ru: 'Доля успешных запросов',
    vi: 'Tỷ lệ thành công',
  },
  'Average TTFT': {
    en: 'Average TTFT',
    zh: '平均首 Token 延迟',
    'zh-TW': '平均首 Token 延遲',
    fr: 'TTFT moyen',
    ja: '平均 TTFT',
    ru: 'Средний TTFT',
    vi: 'TTFT trung bình',
  },
  'Average latency': {
    en: 'Average latency',
    zh: '平均延迟',
    'zh-TW': '平均延遲',
    fr: 'Latence moyenne',
    ja: '平均レイテンシ',
    ru: 'Средняя задержка',
    vi: 'Độ trễ trung bình',
  },
  'No model traffic in this group yet.': {
    en: 'No model traffic in this group yet.',
    zh: '该分组暂无模型流量。',
    'zh-TW': '該分組暫無模型流量。',
    fr: 'Aucun trafic de modèle dans ce groupe pour le moment.',
    ja: 'このグループにはまだモデルのトラフィックがありません。',
    ru: 'В этой группе пока нет трафика моделей.',
    vi: 'Nhóm này chưa có lưu lượng mô hình.',
  },
  'Failed to load performance data.': {
    en: 'Failed to load performance data.',
    zh: '性能数据加载失败。',
    'zh-TW': '效能資料載入失敗。',
    fr: 'Échec du chargement des données de performance.',
    ja: 'パフォーマンスデータの読み込みに失敗しました。',
    ru: 'Не удалось загрузить данные о производительности.',
    vi: 'Không thể tải dữ liệu hiệu năng.',
  },
  Retry: {
    en: 'Retry',
    zh: '重试',
    'zh-TW': '重試',
    fr: 'Réessayer',
    ja: '再試行',
    ru: 'Повторить попытку',
    vi: 'Thử lại',
  },
  'You have no usable groups yet.': {
    en: 'You have no usable groups yet.',
    zh: '当前账号暂无可用分组。',
    'zh-TW': '目前帳號暫無可用分組。',
    fr: "Vous n'avez pas encore de groupes utilisables.",
    ja: '利用可能なグループがまだありません。',
    ru: 'У вас пока нет доступных групп.',
    vi: 'Bạn chưa có nhóm khả dụng nào.',
  },
  'Last hour': {
    en: 'Last hour',
    zh: '最近 1 小时',
    'zh-TW': '最近 1 小時',
    fr: 'Dernière heure',
    ja: '直近 1 時間',
    ru: 'Последний час',
    vi: '1 giờ qua',
  },
}

// Values that exist in every locale but still equal the English key.
const UPDATED_VALUES = {
  'View models': {
    fr: 'Voir les modèles',
    ja: 'モデルを表示',
    ru: 'Показать модели',
    vi: 'Xem mô hình',
  },
}

function escapeJson(value) {
  return JSON.stringify(value)
}

function keyLineIndex(lines, key, from, to) {
  const target = JSON.parse(escapeJson(key))
  for (let i = from; i < to; i += 1) {
    const match = lines[i].match(/^\s*"((?:[^"\\]|\\.)*)"\s*:/)
    if (!match) continue
    if (JSON.parse('"' + match[1] + '"') === target) return i
  }
  return -1
}

async function insertMissingKeys(locale) {
  const file = path.join(LOCALES_DIR, locale + '.json')
  const text = await fs.readFile(file, 'utf8')
  const parsed = JSON.parse(text)
  const existing = parsed.translation ?? {}
  const pending = Object.keys(NEW_KEYS)
    .filter((key) => NEW_KEYS[key][locale] && existing[key] === undefined)
    .sort((a, b) => a.localeCompare(b, 'en'))
  const updatesNeeded = Object.keys(UPDATED_VALUES).some(
    (key) => UPDATED_VALUES[key][locale] && existing[key] === key
  )
  if (pending.length === 0 && !updatesNeeded) return 0

  const lines = text.split('\n')
  const translationStart = lines.findIndex((line) =>
    /"translation"\s*:\s*\{/.test(line)
  )
  if (translationStart < 0)
    throw new Error('translation object not found in ' + file)
  let end = lines.length - 1
  for (let i = translationStart + 1; i < lines.length; i += 1) {
    if (/^\s*\}\s*,?\s*$/.test(lines[i])) {
      end = i
      break
    }
  }

  for (const key of pending) {
    let insertAt = keyLineIndex(lines, key, translationStart + 1, end)
    if (insertAt < 0) {
      // Keep the trailing key unprefixed by inserting after the last key line.
      let lastKeyLine = -1
      for (let i = end - 1; i > translationStart; i -= 1) {
        if (/^\s*"(?:[^"\\]|\\.)*"\s*:/.test(lines[i])) {
          lastKeyLine = i
          break
        }
      }
      if (lastKeyLine < 0)
        throw new Error('no key lines before the end of translation in ' + file)
      // The last key line has no trailing comma: insert before it instead.
      lines.splice(
        lastKeyLine,
        0,
        '    ' +
          escapeJson(key) +
          ': ' +
          escapeJson(NEW_KEYS[key][locale]) +
          ','
      )
      end += 1
      continue
    }
    lines.splice(
      insertAt,
      0,
      '    ' + escapeJson(key) + ': ' + escapeJson(NEW_KEYS[key][locale]) + ','
    )
    end += 1
  }

  for (const key of Object.keys(UPDATED_VALUES)) {
    const value = UPDATED_VALUES[key][locale]
    if (!value || existing[key] !== key) continue
    const at = keyLineIndex(lines, key, translationStart + 1, lines.length - 1)
    if (at < 0) continue
    const comma = lines[at].trimEnd().endsWith(',') ? ',' : ''
    lines[at] = '    ' + escapeJson(key) + ': ' + escapeJson(value) + comma
  }

  await fs.writeFile(file, lines.join('\n'), 'utf8')
  return pending.length
}

async function main() {
  const summary = []
  for (const locale of LOCALES) {
    const added = await insertMissingKeys(locale)
    summary.push(locale + ': ' + added)
  }
  console.log('add-missing-keys done -> ' + summary.join(', '))
}

await main()
