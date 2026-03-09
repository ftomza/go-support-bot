import { useState, useEffect, useRef } from 'react';
import { ChevronRight, ChevronDown, Plus, Trash2, Settings, MessageSquare, Clock } from 'lucide-react';

const tg = window.Telegram?.WebApp;

// Рекурсивный компонент для отрисовки папок и тем
const ThemeNode = ({ name, node, path, onChange, onDelete, onAddSub, managers }) => {
    const [expanded, setExpanded] = useState(false);
    const isLeaf = !node.SubTheme || Object.keys(node.SubTheme).length === 0;

    // Логика для тайм-пикера (разбиваем "09:00-18:00" на start и end)
    const hours = node.WorkHours || '';
    const [startTime = '', endTime = ''] = hours.split('-');

    const handleTimeChange = (type, val) => {
        let newStart = type === 'start' ? val : startTime;
        let newEnd = type === 'end' ? val : endTime;

        // Если оба пустые - очищаем график (24/7)
        if (!newStart && !newEnd) {
            onChange(path, 'WorkHours', '');
        } else {
            // Если заполнили только один, второй ставим по умолчанию
            onChange(path, 'WorkHours', `${newStart || '00:00'}-${newEnd || '23:59'}`);
        }
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
                    <div className="flex gap-2">
                        <button onClick={() => onAddSub(path)} className="text-tg-button p-1" title="Добавить подтему"><Plus size={18}/></button>
                        <button onClick={() => onDelete(path, name)} className="text-red-500 p-1" title="Удалить"><Trash2 size={18}/></button>
                    </div>
                </div>

                {/* Настройки текущей темы */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-2 text-sm">
                    <div className="flex flex-col gap-1">
                        <label className="text-xs text-tg-hint font-semibold">Сообщение при выборе (HTML)</label>
                        <input
                            placeholder="Введите текст..."
                            value={node.Text || ''}
                            onChange={(e) => onChange(path, 'Text', e.target.value)}
                            className="w-full bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link"
                        />
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
                            <Clock size={12}/> Часы работы (оставьте пустым для 24/7)
                        </label>
                        <div className="flex items-center gap-2">
                            <input
                                type="time"
                                value={startTime}
                                onChange={(e) => handleTimeChange('start', e.target.value)}
                                className="bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link flex-1"
                            />
                            <span className="text-tg-hint">—</span>
                            <input
                                type="time"
                                value={endTime}
                                onChange={(e) => handleTimeChange('end', e.target.value)}
                                className="bg-tg-bg border border-tg-hint/30 rounded p-1.5 outline-none focus:border-tg-link flex-1"
                            />
                        </div>
                    </div>
                </div>
            </div>

            {/* Рендерим дочерние элементы */}
            {expanded && node.SubTheme && (
                <div className="mt-2">
                    {Object.entries(node.SubTheme).map(([childName, childNode]) => (
                        <ThemeNode
                            key={childName} name={childName} node={childNode}
                            path={[...path, childName]}
                            onChange={onChange} onDelete={onDelete} onAddSub={onAddSub}
                            managers={managers}
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
    const [activeTab, setActiveTab] = useState('themes');

    useEffect(() => {
        configRef.current = config;
    }, [config]);

    useEffect(() => {
        if (tg) {
            tg.ready();
            tg.expand();
            tg.MainButton.setText('СОХРАНИТЬ КОНФИГУРАЦИЮ');
            tg.MainButton.onClick(saveConfig);
        }
        fetchData();
    }, []);

    useEffect(() => {
        if (config && tg) tg.MainButton.show();
    }, [config]);

    const fetchData = async () => {
        try {
            // Параллельно загружаем конфиг и список менеджеров
            const [configRes, managersRes] = await Promise.all([
                fetch('/api/config/get'),
                fetch('/api/managers')
            ]);

            const configData = await configRes.json();
            const managersData = await managersRes.json();

            if (!configData.Themes) configData.Themes = {};

            setConfig(configData);
            setManagers(managersData || []);
        } catch (err) {
            tg?.showAlert('Ошибка загрузки данных');
        } finally {
            setLoading(false);
        }
    };

    const saveConfig = async () => {
        const currentConfig = configRef.current; // <--- БЕРЕМ СВЕЖИЕ ДАННЫЕ ИЗ REF!
        if (!currentConfig) return;

        tg?.MainButton.showProgress();
        try {
            const res = await fetch('/api/config/save', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Telegram-Init-Data': tg?.initData || ''
                },
                body: JSON.stringify(currentConfig) // <--- ШЛЕМ СВЕЖИЕ ДАННЫЕ
            });
            if (!res.ok) throw new Error('Ошибка сервера');
            tg?.showAlert('Успешно сохранено!');
        } catch (err) {
            tg?.showAlert('Ошибка при сохранении: ' + err.message);
        } finally {
            tg?.MainButton.hideProgress();
        }
    };

    const updateTree = (path, field, value, oldName = null) => {
        setConfig(prev => {
            const newConfig = JSON.parse(JSON.stringify(prev));
            let current = newConfig.Themes;

            for (let i = 0; i < path.length - 1; i++) {
                current = current[path[i]].SubTheme;
            }

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

    const deleteNode = (path, name) => {
        if (!window.confirm(`Удалить тему "${name}"?`)) return;
        setConfig(prev => {
            const newConfig = JSON.parse(JSON.stringify(prev));
            let current = newConfig.Themes;
            for (let i = 0; i < path.length - 1; i++) current = current[path[i]].SubTheme;
            delete current[path[path.length - 1]];
            return newConfig;
        });
    };

    const addSubNode = (path) => {
        const newName = prompt('Введите название новой подтемы:');
        if (!newName) return;
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

            current[newName] = { Text: '', Manager: null, WorkHours: '' };
            return newConfig;
        });
    };

    if (loading) return <div className="p-5 text-center text-tg-hint">Загрузка интерфейса...</div>;

    return (
        <div className="pb-24">
            {/* Tabs */}
            <div className="flex border-b border-tg-hint/30 mb-4 bg-tg-secondaryBg sticky top-0 z-10">
                <button
                    className={`flex-1 p-3 flex items-center justify-center gap-2 font-semibold ${activeTab === 'themes' ? 'border-b-2 border-tg-button text-tg-button' : 'text-tg-hint'}`}
                    onClick={() => setActiveTab('themes')}
                >
                    <Settings size={18}/> Темы
                </button>
                <button
                    className={`flex-1 p-3 flex items-center justify-center gap-2 font-semibold ${activeTab === 'texts' ? 'border-b-2 border-tg-button text-tg-button' : 'text-tg-hint'}`}
                    onClick={() => setActiveTab('texts')}
                >
                    <MessageSquare size={18}/> Тексты
                </button>
            </div>

            <div className="px-4">
                {activeTab === 'themes' && (
                    <div>
                        <div className="flex justify-between items-center mb-4">
                            <h2 className="text-xl font-bold">Дерево категорий</h2>
                            <button onClick={() => addSubNode([])} className="bg-tg-button text-tg-buttonText px-3 py-1.5 rounded flex items-center gap-1 text-sm shadow">
                                <Plus size={16}/> Добавить корень
                            </button>
                        </div>
                        {Object.entries(config.Themes).map(([name, node]) => (
                            <ThemeNode
                                key={name} name={name} node={node} path={[name]}
                                onChange={updateTree} onDelete={deleteNode} onAddSub={addSubNode}
                                managers={managers}
                            />
                        ))}
                    </div>
                )}

                {activeTab === 'texts' && (
                    <div className="flex flex-col gap-4">
                        <h2 className="text-xl font-bold">Системные сообщения</h2>
                        {[
                            { key: 'Text', label: 'Главный вопрос (корень)', parent: config },
                            { key: 'WelcomeNewUser', label: 'Приветствие новичка', parent: config.Messages },
                            { key: 'AskForText', label: 'Просьба писать текстом', parent: config.Messages },
                            { key: 'TopicCreated', label: 'Успешное создание топика', parent: config.Messages },
                            { key: 'OutOfHours', label: 'Нерабочее время (%s - подстановка часов)', parent: config.Messages },
                            { key: 'ServerError', label: 'Ошибка сервера', parent: config.Messages },
                        ].map(({ key, label, parent }) => (
                            <div key={key} className="flex flex-col gap-1">
                                <label className="text-sm font-semibold text-tg-hint">{label}</label>
                                <textarea
                                    value={parent[key] || ''}
                                    onChange={(e) => {
                                        const val = e.target.value;
                                        setConfig(prev => {
                                            const next = {...prev};
                                            if (key === 'Text') next[key] = val;
                                            else next.Messages[key] = val;
                                            return next;
                                        });
                                    }}
                                    className="w-full bg-tg-secondaryBg border border-transparent rounded p-2 min-h-[80px] outline-none focus:border-tg-link transition-colors"
                                />
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {!tg?.initData && (
                <div className="fixed bottom-0 left-0 right-0 p-4 bg-tg-bg border-t border-tg-hint/30">
                    <button onClick={saveConfig} className="w-full bg-tg-button text-tg-buttonText py-3 rounded-xl font-bold shadow-lg">
                        Сохранить конфигурацию (Dev)
                    </button>
                </div>
            )}
        </div>
    );
}