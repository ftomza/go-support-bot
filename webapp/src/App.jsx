import { useState, useEffect, useRef } from 'react';
import { ChevronRight, ChevronDown, Plus, Trash2, Settings, MessageSquare, Clock, ArrowUp, ArrowDown, Users, Send, Search, RotateCcw, AlertCircle } from 'lucide-react';

const tg = window.Telegram?.WebApp;

const ThemeNode = ({ name, node, path, onChange, onDelete, onAddSub, onMove, isFirst, isLast, managers }) => {
    const [expanded, setExpanded] = useState(false);
    const isLeaf = !node.SubTheme || Object.keys(node.SubTheme).length === 0;

    const hours = node.WorkHours || '';
    const [startTime = '', endTime = ''] = hours.split('-');

    const handleTimeChange = (type, val) => {
        let newStart = type === 'start' ? val : startTime;
        let newEnd = type === 'end' ? val : endTime;
        if (!newStart && !newEnd) onChange(path, 'WorkHours', '');
        else onChange(path, 'WorkHours', `${newStart || '00:00'}-${newEnd || '23:59'}`);
    };

    return (
        <div className="ml-4 mt-2 border-l-2 border-tg-hint/30 pl-4">
            <div className="flex flex-col gap-2 bg-tg-secondaryBg p-3 rounded-lg shadow-sm">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 cursor-pointer font-semibold" onClick={() => setExpanded(!expanded)}>
                        {!isLeaf ? (expanded ? <ChevronDown size={18}/> : <ChevronRight size={18}/>) : <div className="w-[18px]"/>}
                        <input
                            value={name}
                            onChange={(e) => onChange(path, 'rename', e.target.value, name)}
                            className="bg-transparent border-b border-transparent focus:border-tg-link outline-none"
                            onClick={(e) => e.stopPropagation()}
                        />
                    </div>
                    <div className="flex gap-1 items-center">
                        {onMove && (
                            <div className="flex bg-tg-bg border border-tg-hint/30 rounded mr-2 h-[28px]">
                                <button onClick={() => onMove(path, -1)} disabled={isFirst} className={`px-1.5 border-r border-tg-hint/30 flex items-center justify-center ${isFirst ? 'text-tg-hint/30' : 'text-tg-button hover:bg-tg-secondaryBg'}`} title="Вверх"><ArrowUp size={16}/></button>
                                <button onClick={() => onMove(path, 1)} disabled={isLast} className={`px-1.5 flex items-center justify-center ${isLast ? 'text-tg-hint/30' : 'text-tg-button hover:bg-tg-secondaryBg'}`} title="Вниз"><ArrowDown size={16}/></button>
                            </div>
                        )}
                        <button onClick={() => onAddSub(path)} className="text-tg-button p-1 hover:bg-tg-secondaryBg rounded" title="Добавить подтему"><Plus size={18}/></button>
                        <button onClick={() => onDelete(path, name)} className="text-red-500 p-1 hover:bg-red-50 rounded" title="Удалить"><Trash2 size={18}/></button>
                    </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-2 text-sm">
                    <div className="flex flex-col gap-1">
                        <label className="text-xs text-tg-hint font-semibold">Сообщение при выборе (HTML)</label>
                        <textarea
                            placeholder="Введите текст..."
                            value={node.Text || ''}
                            onChange={(e) => onChange(path, 'Text', e.target.value)}
                            rows={3}
                            className="w-full bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link resize-y min-h-[60px]"
                        />
                    </div>

                    <div className="flex flex-col gap-1">
                        <label className="text-xs text-tg-hint font-semibold">Картинка (URL)</label>
                        <input
                            type="text"
                            placeholder="https://example.com/image.jpg"
                            value={node.Image || ''}
                            onChange={(e) => onChange(path, 'Image', e.target.value)}
                            className="w-full bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link"
                        />
                        {node.Image && (
                            <img src={node.Image} alt="preview" className="mt-1 h-12 object-cover rounded border border-tg-hint/30" onError={(e) => e.target.style.display='none'} />
                        )}
                    </div>

                    <div className="flex flex-col gap-1">
                        <label className="text-xs text-tg-hint font-semibold">Менеджер</label>
                        <select
                            value={node.Manager || ''}
                            onChange={(e) => onChange(path, 'Manager', e.target.value ? Number(e.target.value) : null)}
                            className="w-full bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link"
                        >
                            <option value="">📁 Без менеджера (Папка)</option>
                            {managers.map(m => (
                                <option key={m.id} value={m.id}>
                                    👤 {m.name} {m.username ? `(@${m.username})` : ''}
                                </option>
                            ))}
                        </select>
                    </div>
                    <div className="flex flex-col gap-1 md:col-span-2">
                        <label className="text-xs text-tg-hint font-semibold flex items-center gap-1">
                            <Clock size={12}/> Часы работы (пусто = 24/7)
                        </label>
                        <div className="flex items-center gap-2 flex-wrap">
                            <input type="time" value={startTime} onChange={(e) => handleTimeChange('start', e.target.value)} className="bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link flex-1 min-w-[100px]" />
                            <span className="text-tg-hint">—</span>
                            <input type="time" value={endTime} onChange={(e) => handleTimeChange('end', e.target.value)} className="bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link flex-1 min-w-[100px]" />

                            <select
                                value={node.Timezone || 'UTC'}
                                onChange={(e) => onChange(path, 'Timezone', e.target.value)}
                                className="bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link flex-1 min-w-[120px]"
                            >
                                <option value="UTC">UTC</option>
                                <option value="Europe/Moscow">Europe/Moscow</option>
                                <option value="Europe/Kiev">Europe/Kiev</option>
                                <option value="Asia/Dubai">Asia/Dubai</option>
                                <option value="Asia/Tbilisi">Asia/Tbilisi</option>
                                <option value="Asia/Almaty">Asia/Almaty</option>
                                <option value="Asia/Yerevan">Asia/Yerevan</option>
                                <option value="Asia/Bangkok">Asia/Bangkok</option>
                                <option value="Europe/London">Europe/London</option>
                                <option value="America/New_York">America/New_York</option>
                            </select>
                        </div>
                    </div>
                </div>
            </div>

            {expanded && node.SubTheme && (
                <div className="mt-2">
                    {Object.entries(node.SubTheme)
                        .sort((a, b) => (a[1].Order || 0) - (b[1].Order || 0))
                        .map(([childName, childNode], index, arr) => (
                            <ThemeNode
                                key={childName} name={childName} node={childNode} path={[...path, childName]}
                                onChange={onChange} onDelete={onDelete} onAddSub={onAddSub} onMove={onMove}
                                isFirst={index === 0} isLast={index === arr.length - 1} managers={managers}
                            />
                        ))}
                </div>
            )}
        </div>
    );
};

export default function App() {
    const [config, setConfig] = useState(null);
    const configRef = useRef(null);
    const [managers, setManagers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [activeTab, setActiveTab] = useState('themes'); // themes, texts, broadcasts
    const [promptModal, setPromptModal] = useState({ isOpen: false, path: null, value: '' });

    // Состояния для вкладки Рассылок
    const [customers, setCustomers] = useState([]);
    const [broadcastHistory, setBroadcastHistory] = useState([]);
    const [selectedCustomers, setSelectedCustomers] = useState(new Set());
    const [broadcastText, setBroadcastText] = useState('');
    const [searchQuery, setSearchQuery] = useState('');
    const [isSending, setIsSending] = useState(false);

    useEffect(() => { configRef.current = config; }, [config]);

    useEffect(() => {
        if (tg) {
            tg.ready();
            tg.expand();
            tg.MainButton.setText('СОХРАНИТЬ КОНФИГУРАЦИЮ');
            tg.MainButton.onClick(saveConfig);
        }
        fetchData();
    }, []);

    // Управляем видимостью главной кнопки Telegram
    useEffect(() => {
        if (tg) {
            if (activeTab === 'broadcasts') {
                tg.MainButton.hide(); // Скрываем кнопку сохранения на вкладке рассылок
            } else if (config) {
                tg.MainButton.show();
            }
        }
    }, [activeTab, config]);

    // Загрузка данных для вкладки Рассылок
    useEffect(() => {
        if (activeTab === 'broadcasts') {
            fetchBroadcastData();
        }
    }, [activeTab]);

    const fetchData = async () => {
        try {
            const headers = { 'X-Telegram-Init-Data': tg?.initData || '' };
            const [configRes, managersRes] = await Promise.all([
                fetch('/api/config/get', { headers }),
                fetch('/api/managers', { headers })
            ]);

            if (!configRes.ok || !managersRes.ok) throw new Error('Ошибка авторизации или сервера');

            const configData = await configRes.json();
            const managersData = await managersRes.json();

            if (!configData.Themes) configData.Themes = {};
            setConfig(configData);
            setManagers(managersData || []);
        } catch (err) {
            tg?.showAlert('Ошибка загрузки данных: ' + err.message);
        } finally {
            setLoading(false);
        }
    };

    const fetchBroadcastData = async (search = '') => {
        try {
            const headers = { 'X-Telegram-Init-Data': tg?.initData || '' };
            const [custRes, histRes] = await Promise.all([
                fetch(`/api/customers?search=${encodeURIComponent(search)}`, { headers }),
                fetch(`/api/broadcasts/history`, { headers })
            ]);

            if (custRes.ok) setCustomers(await custRes.json() || []);
            if (histRes.ok) setBroadcastHistory(await histRes.json() || []);
        } catch (err) {
            console.error('Ошибка загрузки данных рассылки:', err);
        }
    };

    const saveConfig = async () => {
        const currentConfig = configRef.current;
        if (!currentConfig) return;

        tg?.MainButton.showProgress();
        try {
            const res = await fetch('/api/config/save', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-Telegram-Init-Data': tg?.initData || '' },
                body: JSON.stringify(currentConfig)
            });
            if (!res.ok) throw new Error('Ошибка сервера');
            tg?.showAlert('Успешно сохранено!');
        } catch (err) {
            tg?.showAlert('Ошибка при сохранении: ' + err.message);
        } finally {
            tg?.MainButton.hideProgress();
        }
    };

    // --- Обработчики для Рассылок ---
    const toggleCustomer = (id) => {
        setSelectedCustomers(prev => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });
    };

    const toggleAllVisible = () => {
        const validCustomers = customers.filter(c => !c.is_blocked);
        if (selectedCustomers.size === validCustomers.length) {
            setSelectedCustomers(new Set()); // Снять все
        } else {
            setSelectedCustomers(new Set(validCustomers.map(c => c.customer_id))); // Выбрать всех доступных
        }
    };

    const handleSendBroadcast = async () => {
        if (!broadcastText.trim()) return tg?.showAlert('Введите текст рассылки');
        if (selectedCustomers.size === 0) return tg?.showAlert('Выберите получателей');

        if (!window.confirm(`Запустить рассылку для ${selectedCustomers.size} пользователей?`)) return;

        setIsSending(true);
        try {
            const res = await fetch('/api/broadcasts/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-Telegram-Init-Data': tg?.initData || '' },
                body: JSON.stringify({
                    text: broadcastText,
                    customer_ids: Array.from(selectedCustomers)
                })
            });
            if (!res.ok) throw new Error('Ошибка сервера');

            tg?.showAlert('Рассылка успешно добавлена в очередь!');
            setBroadcastText('');
            setSelectedCustomers(new Set());
            fetchBroadcastData(searchQuery); // Обновляем историю
        } catch (err) {
            tg?.showAlert('Ошибка при запуске рассылки: ' + err.message);
        } finally {
            setIsSending(false);
        }
    };

    const handleRetryBroadcast = async (broadcastId) => {
        if (!window.confirm('Повторить отправку всем, кому сообщение не дошло?')) return;

        try {
            const res = await fetch('/api/broadcasts/retry', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-Telegram-Init-Data': tg?.initData || '' },
                body: JSON.stringify({ broadcast_id: broadcastId })
            });
            if (!res.ok) throw new Error('Ошибка сервера');

            tg?.showAlert('Сообщения с ошибками возвращены в очередь!');
            fetchBroadcastData(searchQuery);
        } catch (err) {
            tg?.showAlert('Ошибка: ' + err.message);
        }
    };
    // --------------------------------

    const updateTree = (path, field, value, oldName = null) => {
        setConfig(prev => {
            const newConfig = JSON.parse(JSON.stringify(prev));
            let current = newConfig.Themes;
            for (let i = 0; i < path.length - 1; i++) current = current[path[i]].SubTheme;
            const targetName = path[path.length - 1];
            if (field === 'rename') {
                if (value && value !== oldName) {
                    current[value] = current[oldName];
                    delete current[oldName];
                }
            } else {
                current[targetName][field] = value;
            }
            return newConfig;
        });
    };

    const moveNode = (path, direction) => {
        setConfig(prev => {
            const newConfig = JSON.parse(JSON.stringify(prev));
            let parent = newConfig.Themes;
            for (let i = 0; i < path.length - 1; i++) parent = parent[path[i]].SubTheme;

            const targetName = path[path.length - 1];

            const siblings = Object.entries(parent).map(([k, v], idx) => {
                if (v.Order === undefined) v.Order = idx;
                return [k, v];
            }).sort((a, b) => a[1].Order - b[1].Order);

            const currentIndex = siblings.findIndex(s => s[0] === targetName);
            if (currentIndex === -1) return newConfig;

            if (direction === -1 && currentIndex > 0) {
                const prevName = siblings[currentIndex - 1][0];
                const temp = parent[targetName].Order;
                parent[targetName].Order = parent[prevName].Order;
                parent[prevName].Order = temp;
            } else if (direction === 1 && currentIndex < siblings.length - 1) {
                const nextName = siblings[currentIndex + 1][0];
                const temp = parent[targetName].Order;
                parent[targetName].Order = parent[nextName].Order;
                parent[nextName].Order = temp;
            }
            return newConfig;
        });
    };

    const requestDelete = (path, name) => {
        if (tg && tg.showPopup) {
            tg.showPopup({
                title: 'Удаление', message: `Удалить тему "${name}"?`,
                buttons: [{ id: 'delete', type: 'destructive', text: 'Удалить' }, { type: 'cancel' }]
            }, (btnId) => { if (btnId === 'delete') performDelete(path); });
        } else {
            if (window.confirm(`Удалить тему "${name}"?`)) performDelete(path);
        }
    };

    const performDelete = (path) => {
        setConfig(prev => {
            const newConfig = JSON.parse(JSON.stringify(prev));
            let current = newConfig.Themes;
            for (let i = 0; i < path.length - 1; i++) current = current[path[i]].SubTheme;
            delete current[path[path.length - 1]];
            return newConfig;
        });
    };

    const openAddModal = (path) => { setPromptModal({ isOpen: true, path, value: '' }); };

    const submitAddModal = () => {
        const { path, value } = promptModal;
        const newName = value.trim();
        if (!newName) return setPromptModal({ isOpen: false, path: null, value: '' });

        setConfig(prev => {
            const newConfig = JSON.parse(JSON.stringify(prev));
            let current = path.length === 0 ? newConfig.Themes : newConfig.Themes;

            if (path.length > 0) {
                for (let i = 0; i < path.length; i++) {
                    if (i === path.length - 1) {
                        if (!current[path[i]].SubTheme) current[path[i]].SubTheme = {};
                        current = current[path[i]].SubTheme;
                    } else {
                        current = current[path[i]].SubTheme;
                    }
                }
            }
            const newOrder = Object.keys(current).length;
            current[newName] = { Text: '', Image: '', Manager: null, WorkHours: '', Timezone: 'UTC', Order: newOrder };
            return newConfig;
        });
        setPromptModal({ isOpen: false, path: null, value: '' });
    };

    if (loading) return <div className="p-5 text-center text-tg-hint">Загрузка интерфейса...</div>;

    return (
        <div className="pb-24 relative">
            {promptModal.isOpen && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
                    <div className="bg-tg-bg w-full max-w-sm rounded-2xl p-5 shadow-xl border border-tg-hint/20">
                        <h3 className="text-lg font-bold mb-3 text-tg-text">Название новой темы</h3>
                        <input autoFocus type="text" value={promptModal.value} onChange={e => setPromptModal(prev => ({ ...prev, value: e.target.value }))} placeholder="Введите название..." className="w-full bg-tg-secondaryBg border border-tg-hint/30 rounded-xl p-3 mb-5 outline-none focus:border-tg-link text-tg-text transition-colors" onKeyDown={e => e.key === 'Enter' && submitAddModal()}/>
                        <div className="flex justify-end gap-2">
                            <button onClick={() => setPromptModal({ isOpen: false, path: null, value: '' })} className="px-4 py-2.5 text-tg-hint font-medium hover:bg-tg-secondaryBg rounded-xl transition-colors">Отмена</button>
                            <button onClick={submitAddModal} className="px-4 py-2.5 bg-tg-button text-tg-buttonText font-bold rounded-xl shadow-sm transition-opacity hover:opacity-90">Добавить</button>
                        </div>
                    </div>
                </div>
            )}

            <div className="flex border-b border-tg-hint/30 mb-4 bg-tg-secondaryBg sticky top-0 z-10 shadow-sm">
                <button className={`flex-1 p-3 flex items-center justify-center gap-2 font-semibold ${activeTab === 'themes' ? 'border-b-2 border-tg-button text-tg-button' : 'text-tg-hint'}`} onClick={() => setActiveTab('themes')}><Settings size={18}/> Темы</button>
                <button className={`flex-1 p-3 flex items-center justify-center gap-2 font-semibold ${activeTab === 'texts' ? 'border-b-2 border-tg-button text-tg-button' : 'text-tg-hint'}`} onClick={() => setActiveTab('texts')}><MessageSquare size={18}/> Тексты</button>
                <button className={`flex-1 p-3 flex items-center justify-center gap-2 font-semibold ${activeTab === 'broadcasts' ? 'border-b-2 border-tg-button text-tg-button' : 'text-tg-hint'}`} onClick={() => setActiveTab('broadcasts')}><Users size={18}/> Рассылка</button>
            </div>

            <div className="px-4">
                {activeTab === 'themes' && (
                    <div>
                        <div className="flex justify-between items-center mb-4">
                            <h2 className="text-xl font-bold">Дерево категорий</h2>
                            <button onClick={() => openAddModal([])} className="bg-tg-button text-tg-buttonText px-3 py-1.5 rounded-lg flex items-center gap-1 text-sm shadow"><Plus size={16}/> Корень</button>
                        </div>
                        {Object.entries(config.Themes)
                            .sort((a, b) => (a[1].Order || 0) - (b[1].Order || 0))
                            .map(([name, node], index, arr) => (
                                <ThemeNode
                                    key={name} name={name} node={node} path={[name]}
                                    onChange={updateTree} onDelete={requestDelete} onAddSub={openAddModal} onMove={moveNode}
                                    isFirst={index === 0} isLast={index === arr.length - 1} managers={managers}
                                />
                            ))}
                    </div>
                )}

                {activeTab === 'texts' && (
                    <div className="flex flex-col gap-6 pb-6">
                        <div className="bg-tg-secondaryBg p-4 rounded-xl shadow-sm border border-tg-hint/20">
                            <h2 className="text-xl font-bold mb-4 text-tg-text">Сообщения для клиентов</h2>
                            <div className="flex flex-col gap-4">
                                {[
                                    { key: 'Text', label: 'Главный вопрос в начале', parent: config },
                                    { key: 'WelcomeNewUser', label: 'Приветствие новичка (спрашиваем имя)', parent: config.Messages },
                                    { key: 'AskForText', label: 'Если прислали не текст вместо имени', parent: config.Messages },
                                    { key: 'SelectTopic', label: 'Дефолтный текст меню "Выберите тему"', parent: config.Messages },
                                    { key: 'SelectSubtopic', label: 'Дефолтный текст меню "Выберите подтему"', parent: config.Messages },
                                    { key: 'TopicCreated', label: 'Успешное создание обращения', parent: config.Messages },
                                    { key: 'OutOfHours', label: 'Нерабочее время (%s - подстановка часов)', parent: config.Messages },
                                    { key: 'RateService', label: 'Просьба оценить качество (NPS)', parent: config.Messages },
                                    { key: 'RatingThanks', label: 'Ответ после оценки NPS', parent: config.Messages },
                                    { key: 'TopicClosedByManager', label: 'Топик закрыт менеджером', parent: config.Messages },
                                    { key: 'TopicClosedByClient', label: 'Топик завершен самим клиентом', parent: config.Messages },
                                    { key: 'PromptNewQuestions', label: 'Вопрос после закрытия тикета', parent: config.Messages },
                                    { key: 'PromptReturn', label: 'При возвращении в закрытый топик', parent: config.Messages },
                                    { key: 'TopicAlreadyClosed', label: 'Ошибка: Обращение уже закрыто', parent: config.Messages },
                                    { key: 'CloseTopicButton', label: 'Текст на кнопке [Завершить]', parent: config.Messages },
                                    { key: 'ButtonBack', label: 'Кнопка [Назад]', parent: config.Messages },
                                    { key: 'ButtonHome', label: 'Кнопка [В начало]', parent: config.Messages },
                                ].map(({ key, label, parent }) => (
                                    <div key={key} className="flex flex-col gap-1">
                                        <label className="text-sm font-semibold text-tg-hint">{label}</label>
                                        <textarea
                                            value={parent[key] || ''}
                                            onChange={(e) => {
                                                const val = e.target.value;
                                                setConfig(prev => { const next = {...prev}; if (key === 'Text') next[key] = val; else next.Messages[key] = val; return next; });
                                            }}
                                            className="w-full bg-tg-bg border border-tg-hint/30 rounded-xl p-3 min-h-[50px] outline-none focus:border-tg-link transition-colors"
                                        />
                                    </div>
                                ))}
                            </div>
                        </div>

                        <div className="bg-tg-secondaryBg p-4 rounded-xl shadow-sm border border-tg-hint/20">
                            <h2 className="text-xl font-bold mb-4 text-tg-text">Уведомления менеджерам</h2>
                            <div className="flex flex-col gap-4">
                                {[
                                    { key: 'NotifyManagerNew', label: 'В ЛС при новом тикете', parent: config.Messages },
                                    { key: 'NotifyTopicCreated', label: 'В топик при открытии', parent: config.Messages },
                                    { key: 'NotifyTopicClosedClient', label: 'В топик: клиент сам завершил', parent: config.Messages },
                                    { key: 'NotifyTopicClosedManager', label: 'В топик: менеджер закрыл', parent: config.Messages },
                                    { key: 'NotifyTopicRecreated', label: 'В топик при пересоздании', parent: config.Messages },
                                    { key: 'ServerError', label: 'Текст при внутренней ошибке', parent: config.Messages },
                                ].map(({ key, label, parent }) => (
                                    <div key={key} className="flex flex-col gap-1">
                                        <label className="text-sm font-semibold text-tg-hint">{label}</label>
                                        <textarea
                                            value={parent[key] || ''}
                                            onChange={(e) => {
                                                const val = e.target.value;
                                                setConfig(prev => { const next = {...prev}; next.Messages[key] = val; return next; });
                                            }}
                                            className="w-full bg-tg-bg border border-tg-hint/30 rounded-xl p-3 min-h-[50px] outline-none focus:border-tg-link transition-colors font-mono text-sm"
                                        />
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                )}

                {activeTab === 'broadcasts' && (
                    <div className="flex flex-col gap-6 pb-6">

                        {/* Создание рассылки */}
                        <div className="bg-tg-secondaryBg p-4 rounded-xl shadow-sm border border-tg-hint/20">
                            <h2 className="text-xl font-bold mb-4 text-tg-text flex items-center gap-2">
                                <Send size={20}/> Новое сообщение
                            </h2>

                            {/* Поиск и список клиентов */}
                            <div className="mb-4">
                                <div className="flex gap-2 mb-3">
                                    <div className="relative flex-1">
                                        <Search className="absolute left-3 top-2.5 text-tg-hint" size={18} />
                                        <input
                                            type="text"
                                            placeholder="Поиск по имени или ID..."
                                            value={searchQuery}
                                            onChange={(e) => {
                                                setSearchQuery(e.target.value);
                                                fetchBroadcastData(e.target.value);
                                            }}
                                            className="w-full bg-tg-bg border border-tg-hint/30 rounded-xl py-2 pl-10 pr-3 outline-none focus:border-tg-link transition-colors"
                                        />
                                    </div>
                                </div>

                                <div className="border border-tg-hint/30 rounded-xl overflow-hidden bg-tg-bg">
                                    <div className="flex items-center justify-between p-3 border-b border-tg-hint/30 bg-tg-secondaryBg">
                                        <label className="flex items-center gap-2 cursor-pointer font-semibold text-sm">
                                            <input
                                                type="checkbox"
                                                checked={selectedCustomers.size > 0 && selectedCustomers.size === customers.filter(c => !c.is_blocked).length}
                                                onChange={toggleAllVisible}
                                                className="w-4 h-4 rounded"
                                            />
                                            Выбрать всех ({customers.filter(c => !c.is_blocked).length})
                                        </label>
                                        <span className="text-xs text-tg-hint">Выбрано: {selectedCustomers.size}</span>
                                    </div>
                                    <div className="max-h-48 overflow-y-auto p-2">
                                        {customers.length === 0 ? (
                                            <div className="text-center text-tg-hint text-sm py-4">Клиенты не найдены</div>
                                        ) : (
                                            customers.map(c => (
                                                <label key={c.customer_id} className={`flex items-center justify-between p-2 rounded-lg cursor-pointer hover:bg-tg-secondaryBg transition-colors ${c.is_blocked ? 'opacity-50' : ''}`}>
                                                    <div className="flex items-center gap-3">
                                                        <input
                                                            type="checkbox"
                                                            checked={selectedCustomers.has(c.customer_id)}
                                                            onChange={() => toggleCustomer(c.customer_id)}
                                                            disabled={c.is_blocked}
                                                            className="w-4 h-4 rounded"
                                                        />
                                                        <span className="text-sm font-medium">{c.full_name || `ID: ${c.customer_id}`}</span>
                                                    </div>
                                                    {c.is_blocked && <AlertCircle size={16} className="text-red-500" title="Бот заблокирован" />}
                                                </label>
                                            ))
                                        )}
                                    </div>
                                </div>
                            </div>

                            {/* Текст рассылки */}
                            <div className="flex flex-col gap-2 mb-4">
                                <label className="text-sm font-semibold text-tg-hint">Текст сообщения (поддерживается HTML)</label>
                                <textarea
                                    placeholder="<b>Внимание акция!</b> Приходите к нам..."
                                    value={broadcastText}
                                    onChange={(e) => setBroadcastText(e.target.value)}
                                    className="w-full bg-tg-bg border border-tg-hint/30 rounded-xl p-3 min-h-[100px] outline-none focus:border-tg-link transition-colors"
                                />
                            </div>

                            <button
                                onClick={handleSendBroadcast}
                                disabled={isSending || selectedCustomers.size === 0 || !broadcastText.trim()}
                                className="w-full bg-tg-button text-tg-buttonText py-3 rounded-xl font-bold shadow flex items-center justify-center gap-2 disabled:opacity-50 transition-opacity"
                            >
                                <Send size={18}/> {isSending ? 'Запуск...' : 'Запустить рассылку'}
                            </button>
                        </div>

                        {/* История рассылок */}
                        <div className="bg-tg-secondaryBg p-4 rounded-xl shadow-sm border border-tg-hint/20">
                            <h2 className="text-xl font-bold mb-4 text-tg-text flex items-center justify-between">
                                История рассылок
                                <button onClick={() => fetchBroadcastData(searchQuery)} className="text-tg-button text-sm font-normal hover:underline">Обновить</button>
                            </h2>

                            <div className="flex flex-col gap-3">
                                {broadcastHistory.length === 0 ? (
                                    <div className="text-center text-tg-hint py-4">История пуста</div>
                                ) : (
                                    broadcastHistory.map(b => (
                                        <div key={b.id} className="bg-tg-bg border border-tg-hint/30 rounded-xl p-3 flex flex-col gap-2">
                                            <div className="flex justify-between items-start">
                                                <span className="text-xs text-tg-hint font-mono">{new Date(b.created_at).toLocaleString()}</span>
                                                <span className={`text-xs px-2 py-0.5 rounded-full font-semibold ${b.status === 'completed' ? 'bg-green-100 text-green-700' : 'bg-blue-100 text-blue-700'}`}>
                                                    {b.status === 'completed' ? 'Завершено' : 'В процессе'}
                                                </span>
                                            </div>

                                            <p className="text-sm line-clamp-2 break-words" title={b.text}>{b.text}</p>

                                            <div className="flex items-center gap-3 mt-1 pt-2 border-t border-tg-hint/10 text-xs font-medium">
                                                <span className="text-tg-text">Всего: {b.total}</span>
                                                <span className="text-green-500">✓ {b.sent}</span>
                                                {b.pending > 0 && <span className="text-blue-500">⏳ {b.pending}</span>}
                                                {b.failed > 0 && <span className="text-red-500">❌ {b.failed}</span>}
                                            </div>

                                            {b.failed > 0 && b.status === 'completed' && (
                                                <button
                                                    onClick={() => handleRetryBroadcast(b.id)}
                                                    className="mt-2 text-xs flex items-center justify-center gap-1 bg-red-50 text-red-600 py-1.5 rounded-lg border border-red-200 hover:bg-red-100 transition-colors"
                                                >
                                                    <RotateCcw size={14}/> Повторить отправку для ошибок ({b.failed})
                                                </button>
                                            )}
                                        </div>
                                    ))
                                )}
                            </div>
                        </div>

                    </div>
                )}
            </div>

            {!tg?.initData && activeTab !== 'broadcasts' && (
                <div className="fixed bottom-0 left-0 right-0 p-4 bg-tg-bg border-t border-tg-hint/30">
                    <button onClick={saveConfig} className="w-full bg-tg-button text-tg-buttonText py-3 rounded-xl font-bold shadow-lg">Сохранить (Dev)</button>
                </div>
            )}
        </div>
    );
}