// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

import * as p from '/static/preact-10.18.1.module.js';
import { useEffect, useState, useCallback } from '/static/preact-hooks-10.18.1.module.js';

// API to registry itself
// --------------------------------------------------------------------

async function registryReq(reqKind, req) {
    const response = await fetch(`/api/registry/${reqKind}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(req),
    });

    if (!response.ok) {
        throw new Error(`Bad HTTP status ${response.status}`);
    }

    const r = await response.json();
    return r.resp;
}

function idToShard(id) {
    return Number.parseInt(id.slice(-2), 16);
}

const NULL_INODE_ID = '0x0000000000000000';

function idToType(id) {
    const idN = BigInt(id);
    const typ = (idN >> 61n) & 3n;
    switch (typ) {
    case 1n:
        return 'DIRECTORY';
    case 2n:
        return 'FILE';
    case 3n:
        return 'SYMLINK';
    default:
        throw new Error(`Bad inode id ${id}`);
    }
}

async function shardReq(reqKind, shardId, req) {
    const response = await fetch(`/api/shard/${shardId}/${reqKind}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(req),
    });

    if (!response.ok) {
        throw new Error(`Bad HTTP status ${response.status}`);
    }

    const r = await response.json();
    return r.resp;
}

// Table viewer
// --------------------------------------------------------------------

function rowCells(row) {
    if (Array.isArray(row)) { return row; }
    return row.cells;
}

function rowProps(row) {
    if (Array.isArray(row)) { return {}; }
    return row.props || {};
}

function TableInner(props) {
    const { cols, rows, sortByIx: initialSortByIx, elementsPerPage: initialElementsPerPage } = props
    const paginateExtra = props.paginateExtra || [];
    const disableSearch = !!props.disableSearch;
    const filterFromSearchParams = !!props.filterFromSearchParams;

    function computePages(sortByIx, colFilters, elementsPerPage) {
        const colRegexps = colFilters.map(s => new RegExp(s));
        const filteredRows = rows.filter(row => rowCells(row).every((cell, ix) => {
            let s;
            let col = cols[ix];
            if (col.string) {
                s = col.string(cell);
            } else if (typeof cell !== 'string') {
                s = `${cell}`;
            } else {
                s = cell;
            }
            return s.match(colRegexps[ix]);
        }));
        const sortedRows = filteredRows.sort((row1, row2) => {
            for (const [ix, ascending] of sortByIx) {
                if (rowCells(row1)[ix] < rowCells(row2)[ix]) {
                    return ascending ? -1 : 1;
                } else if (rowCells(row1)[ix] > rowCells(row2)[ix]) {
                    return ascending ? 1 : -1;
                }
            }
            return 0;
        });
        const pages = [];
        for (let i = 0; i < sortedRows.length; i += elementsPerPage) {
            pages.push(sortedRows.slice(i, i + elementsPerPage));
        }
        if (pages.length === 0) {
            pages.push([]);
        }
        return { pages, numRows: sortedRows.length };
    }

    const initialColFilters = Array.from({length: cols.length}, () => '');
    if (filterFromSearchParams) {
        cols.forEach(({name}, ix) => {
            const v = new URLSearchParams(window.location.search).get(name);
            if (v !== null) {
                initialColFilters[ix] = v;
            }
        });
    }

    const [{pages, numRows, currentPage, sortByIx, colFilters, elementsPerPage}, setState] = useState({
        pages: [[]],
        numRows: 0,
        currentPage: 0,
        sortByIx: initialSortByIx,
        colFilters: initialColFilters,
        elementsPerPage: initialElementsPerPage,
    });

    // first filtering
    useEffect(() => {
        const {pages, numRows} = computePages(initialSortByIx, initialColFilters, initialElementsPerPage);
        setState(prev => ({...prev, pages, numRows}));
    }, [])

    const pageRows = pages[currentPage];
    const project = (fld) => (set) => setState(prev => {
        const next = {...prev};
        next[fld] = set(prev[fld]);
        if (fld === 'sortByIx' || fld === 'colFilters' || fld === 'elementsPerPage') {
            if (prev[fld] !== next[fld]) { // deep equal would be nicer
                const {pages, numRows} = computePages(next.sortByIx, next.colFilters, next.elementsPerPage);
                next.pages = pages;
                next.numRows = numRows;
                next.currentPage = 0;
            }
        }
        return next;
    });
    const setSortByIx = project('sortByIx');
    const setColFilters = project('colFilters');
    const setElementsPerPage = project('elementsPerPage');

    if (props.identifier) {
        useEffect(() => {
            localStorage.setItem(`table-${props.identifier}-elements-per-page`, JSON.stringify(elementsPerPage));
            localStorage.setItem(`table-${props.identifier}-sort-by-ix`, JSON.stringify(sortByIx));
        })
    }

    const setIx = (ix) => (ev) => {
        ev.preventDefault();
        setSortByIx(prev => {
            const next = [...prev];
            for (let i = 0; i < next.length; i++) {
                const [otherIx, ascending] = prev[i];
                if (ix === otherIx) {
                    next[i] = [otherIx, !ascending];
                    return next;
                }
            }
            next.push([ix, true]);
            return next;
        });
    };
    const removeIx = (ix) => (ev) => {
        ev.preventDefault();
        setSortByIx(prev => [...prev].filter(([otherIx, _]) => otherIx !== ix));
    };
    const setColFilter = (ix) => (ev) => {
        const v = ev.target.value;
        setColFilters(prev => {
            const next = [...prev];
            next[ix] = v;
            return next;
        });
    };

    const switchPage = (direction) => () => setState(prev => {
        const next = {...prev};
        next.currentPage += direction;
        next.currentPage = Math.max(0, Math.min(next.pages.length-1, next.currentPage));
        return next;
    });

    function Paginate() {
        return p.h('tr', {}, p.h('th', { className: 'input-group-sm py-1', colspan: `${cols.length}` },
            p.h('button', {type: 'button', onClick: switchPage(-1), className: 'btn btn-primary py-1', disabled: currentPage === 0}, '🡐'),
            ' ',
            p.h('button', {type: 'button', onClick: switchPage(+1), className: 'btn btn-primary py-1 me-2', disabled: currentPage >= pages.length-1}, '🡒'),
            ` ${currentPage+1} / ${pages.length} (${numRows} rows)`,
            p.h('input', {
                type: 'number', min: '1', step: '1', className: 'ms-2 form-control', style: 'width: 5rem; display: inline;', value: elementsPerPage,
                onChange: (ev) => {
                    const x = parseInt(ev.target.value);
                    setElementsPerPage(_ => x);
                },
            }),
            ' per page',
            ...paginateExtra,
        ));
    };

    const dummyRows = [];
    if (currentPage > 0) {
        for (let i = pageRows.length; i < elementsPerPage; i++) {
            dummyRows.push(p.h('tr', {}, cols.map(_ => p.h('td', {}, '\u00A0'))));
        }
    }

    return p.h(
        'table', {className: 'table'},
        p.h('thead', {},
            p.h(Paginate),
            p.h('tr', {className: 'text-nowrap'}, cols.map(({name: c}, ix) => {
                const sortIx = sortByIx.findIndex(([otherIx, _]) => ix == otherIx);
                let sortButton;
                if (sortIx < 0) {
                    sortButton = p.h(
                        'a',
                        {
                            className: 'text-muted',
                            href: '#',
                            style: 'text-decoration: none;',
                            onClick: setIx(ix),
                        },
                        '▲'
                    )
                } else {
                    const [_, ascending] = sortByIx[sortIx];
                    sortButton = [
                        p.h('a', { href: '#', style: 'text-decoration: none;', onClick: setIx(ix)}, `${ascending ? "▲" : "▼"}`),
                        p.h('sup', {}, `${sortIx+1}`), ' ',
                        p.h('a', {href: '#', style: 'text-decoration: none;', onClick: removeIx(ix)}, '×')];
                }
                return p.h('th', {}, c, ' ', p.h('small', {}, sortButton))
            })),
            disableSearch ? null :
                p.h('tr', {className: 'text-nowrap'}, cols.map((_, ix) => {
                    return p.h('th', {className: 'py-1 input-group-sm'}, p.h('input', { type: 'text', className: 'form-control', onChange: setColFilter(ix), value: colFilters[ix] }));
                })),
        ),
        p.h('tbody', {},
            pageRows.map(row =>
                p.h('tr', {className: 'text-nowrap', ...rowProps(row)}, rowCells(row).map((r, i) => {
                    let col = cols[i];
                    let content = r;
                    if (col.string) {
                        content = col.string(content);
                    }
                    if (col.render) {
                        content = col.render(content, row);
                    }
                    return p.h('td', {}, content);
                })),
            ),
            dummyRows,
        ),
        pages.length > 1 ? p.h('thead', {}, p.h(Paginate)) : null,
    );
}

function Table(props) {
    const elementsPerPageStorage = localStorage.getItem(`table-${props.identifier}-elements-per-page`);
    const elementsPerPage = elementsPerPageStorage ? parseInt(elementsPerPageStorage) : (props.elementsPerPage || 20);
    const sortByIxStorage = localStorage.getItem(`table-${props.identifier}-sort-by-ix`);
    const sortByIx = sortByIxStorage ? JSON.parse(sortByIxStorage) : [];
    return TableInner({...props, elementsPerPage, sortByIx});
}

// index
// --------------------------------------------------------------------

function stringifyShortFlags(f) {
    if (f == 0) {
        return '0';
    }
    const flags = [[0x1, 'S'], [0x2, 'NR'], [0x4, 'NW'], [0x8, 'D']];
    return flags.filter(([otherF, _]) => otherF&f).map(([_, s]) => s).join('|');
}

function stringifyStorageClass(x) {
    switch (x) {
    case 2:
        return 'HDD';
    case 3:
        return 'FLASH';
    }
    throw Error(`Bad storage ${x}`);
}

function stringifySize(x) {
    let amount = x;
    let unit = 'byte';
    if (x > 1e15) {
        amount = x/1e15;
        unit = 'PB';
    } else if (x > 1e12) {
        amount = x/1e12;
        unit = 'TB';
    } else if (x > 1e9) {
        amount = x/1e9;
        unit = 'GB';
    } else if (x > 1e6) {
        amount = x/1e6;
        unit = 'MB';
    } else if (x > 1e3) {
        amount = x/1e3;
        unit = 'KB';
    }
    return amount.toFixed(2) + unit;
}

function stringifyAgo(then, now) {
    const diffInSeconds = Math.floor((now - then) / 1000);

    let timeAgo;
    if (diffInSeconds < 60) {
        timeAgo = `${diffInSeconds}s ago`;
    } else if (diffInSeconds < 3600) {
        timeAgo = `${Math.floor(diffInSeconds / 60)}m ago`;
    } else if (diffInSeconds < 86400) {
        timeAgo = `${Math.floor(diffInSeconds / 3600)}h ago`;
    } else {
        timeAgo = `${Math.floor(diffInSeconds / 86400)}d ago`;
    }

    return timeAgo;
}

export function renderIndex() {
    function BlockServices() {
        const [blockServices, setBlockServices] = useState(null);
        const [locations, setLocations] = useState(null);
        useEffect(async () => {
            const resp = await registryReq('ALL_BLOCK_SERVICES_DEPRECATED', {});
            setBlockServices(resp);
        }, []);

        useEffect(async () => {
            const resp = await registryReq('LOCATIONS', {});
            var locationsMap = new Map();
            for (const location of resp.Locations) {
                locationsMap.set(location.Id, location.Name);
            }
            setLocations(locationsMap);
        }, []);

        if (blockServices === null || locations === null) {
            return p.h('em', null, 'Loading...');
        }

        const now = new Date();
        const rows = [];
        let failureDomains = new Set();
        let capacity = 0;
        let available = 0;
        let blocks = 0n;
        for (const bs of blockServices.BlockServices) {
            failureDomains.add(bs.FailureDomain);
            capacity += bs.CapacityBytes;
            available += bs.AvailableBytes;
            blocks += BigInt(bs.Blocks);
            rows.push([
                bs.FailureDomain,
                bs.Addrs.Addr1,
                bs.Addrs.Addr2,
                bs.Path,
                bs.Id,
                bs.Flags,
                bs.FlagsLastChanged,
                bs.StorageClass,
                bs.Blocks,
                bs.CapacityBytes,
                bs.AvailableBytes,
                bs.LastSeen,
                bs.HasFiles,
            ]);
        }
        const cols = [
            {name: 'FailureDomain'},
            {name: 'Address 1'},
            {name: 'Address 2'},
            {name: 'Path', render: t => p.h('code', {}, t)},
            {name: 'Id', render: t => p.h('code', {}, t)},
            {name: 'Flags', string: stringifyShortFlags, render: t => p.h('code', {}, t)},
            {name: 'FlagsLastChanged', render: flc => stringifyAgo(new Date(flc), now)},
            {name: 'StorageClass', string: stringifyStorageClass},
            {name: 'Blocks'},
            {name: 'Capacity', string: stringifySize},
            {name: 'Available', string: stringifySize},
            {name: 'LastSeen', render: ls => stringifyAgo(new Date(ls), now)},
            {name: 'HasFiles', string: x => x === true ? "yes" : "no"},
        ];

        return p.h('div', {},
            p.h('p', {}, p.h('ul', {},
                p.h('li', {}, `${rows.length} block services on ${failureDomains.size} failure domains`),
                p.h('li', {}, `Total capacity: ${stringifySize(capacity)}`),
                p.h('li', {}, `${stringifySize(capacity-available)} used over ${blocks} blocks (${(100*(capacity-available)/available).toFixed(2)}%)`),
            )),
            p.h(Table, { identifier: 'block-services', cols, rows, initialSorting: [[0, true], [3, true]] })
        )
    }
    p.render(p.h(BlockServices), document.getElementById('block-services-table'));

    function Shards() {
        const [shards, setShards] = useState(null);
        const [locations, setLocations] = useState(null);
        useEffect(async () => {
            const resp = await registryReq('ALL_SHARDS', {});
            setShards(resp);
        }, []);

        useEffect(async () => {
            const resp = await registryReq('LOCATIONS', {});
            var locationsMap = new Map();
            for (const location of resp.Locations) {
                locationsMap.set(location.Id, location.Name);
            }
            setLocations(locationsMap);
        }, []);

        if (shards === null || locations === null) {
            return p.h('em', null, 'Loading...');
        }

        const now = new Date();
        const rows = [];
        shards.Shards.forEach(shard => {
            rows.push([
                shard.LocationId,
                parseInt(shard.Id.split(':')[0], 10),
                parseInt(shard.Id.split(':')[1], 10),
                shard.IsLeader ? 'yes' : 'no',
                shard.Addrs.Addr1,
                shard.Addrs.Addr2,
                shard.LastSeen,
            ])
        });
        const cols = [
            {name: 'Location', string: i => locations.get(i)},
            {name: 'Id', string: i => i.toString().padStart(3, '0'), render: t => p.h('code', {}, t)},
            {name: 'Replica', render: t => p.h('code', {}, t)},
            {name: 'Leader', render: s => s === 'yes' ? p.h('strong', {}, s) : s},
            {name: 'Address 1', render: t => p.h('code', {}, t)},
            {name: 'Address 2', render: t => p.h('code', {}, t)},
            {name: 'LastSeen', render: ls => stringifyAgo(new Date(ls), now)},
        ];

        return p.h(Table, { identifier: 'shards', cols, rows })
    }
    p.render(p.h(Shards), document.getElementById('shards-table'));

    function Cdc() {
        const [cdcReplicas, setCdcReplicas] = useState(null);
        const [locations, setLocations] = useState(null);

        useEffect(async () => {
            const resp = await registryReq('ALL_CDC', {});
            setCdcReplicas(resp);
        }, []);

        useEffect(async () => {
            const resp = await registryReq('LOCATIONS', {});
            var locationsMap = new Map();
            for (const location of resp.Locations) {
                locationsMap.set(location.Id, location.Name);
            }
            setLocations(locationsMap);
        }, []);

        if (cdcReplicas === null || locations === null) {
            return p.h('em', null, 'Loading...');
        }

        const now = new Date();
        const rows = [];
        cdcReplicas.Replicas.forEach(cdc => {
            rows.push([
                cdc.LocationId,
                cdc.ReplicaId,
                cdc.IsLeader ? 'yes' : 'no',
                cdc.Addrs.Addr1,
                cdc.Addrs.Addr2,
                cdc.LastSeen,
            ])
        });
        const cols = [
            {name: 'Location', string: i => locations.get(i)},
            {name: 'Replica', render: t => p.h('code', {}, t)},
            {name: 'Leader', render: s => s === 'yes' ? p.h('strong', {}, s) : s},
            {name: 'Address 1', render: t => p.h('code', {}, t)},
            {name: 'Address 2', render: t => p.h('code', {}, t)},
            {name: 'LastSeen', render: ls => stringifyAgo(new Date(ls), now)},
        ];

        return p.h(Table, { identifier: 'cdc', cols, rows })
    }
    p.render(p.h(Cdc), document.getElementById('cdc-table'));
}

// browse
// --------------------------------------------------------------------

export function renderDirectoryEdges(id, path) {
    function Edges({id, path, showSnapshotEdges: initialShowSnapshotEdges}) {
        const [{showSnapshotEdges, edges, loadedEdges}, setState] = useState({
            showSnapshotEdges: initialShowSnapshotEdges,
            edges: null,
            loadedEdges: 0,
        });

        useEffect(async () => {
            const results = [];
            const req = {DirId: id};
            while (true) {
                const resp = await shardReq('FULL_READ_DIR', idToShard(id), req);
                results.push(...resp.Results);
                if (resp.Next.StartName === '') {
                    break;
                }
                req.Flags = resp.Next.Current ? 1 : 0;
                req.StartName = resp.Next.StartName;
                req.StartTime = resp.Next.StartTime;
                setState(prev => ({...prev, loadedEdges: results.length}))
            }
            setState(prev => ({...prev, edges: results}));
        }, [])

        useEffect(() => {
            if (showSnapshotEdges) {
                localStorage.setItem('edges-full', showSnapshotEdges);
            } else {
                localStorage.removeItem('edges-full');
            }
        }, [showSnapshotEdges])

        const renderBool = s => s === 'yes' ? p.h('strong', {}, s) : s;

        let edgesTable = p.h('em', {}, `Loading (${loadedEdges}/?)...`);
        if (edges !== null) {
            const rows = [];
            for (const edge of edges) {
                if (!showSnapshotEdges && !edge.Current) {
                    continue;
                }
                rows.push({
                    props: {...(edge.Current ? {} : {className: 'table-active'})},
                    cells: [
                        edge.NameHash,
                        edge.Name,
                        edge.TargetId.Id === NULL_INODE_ID ? '' : idToType(edge.TargetId.Id),
                        edge.TargetId.Id,
                        edge.CreationTime,
                        ...(showSnapshotEdges ? [edge.Current ? 'yes' : 'no'] : []),
                        ...(showSnapshotEdges ? [edge.Current ? 'yes' : (edge.TargetId.Extra ? 'yes' : 'no')] : []),
                        edge.Current ? (edge.TargetId.Extra ? 'yes' : 'no') : '',
                    ],
                })
            }
            const cols = [
                {name: 'NameHash', render: t => p.h('code', {}, t)},
                {
                    name: 'Name',
                    render: (t, row) => {
                        const id = rowCells(row)[3];
                        if (id === NULL_INODE_ID) {
                            return p.h('code', {}, t);
                        } else {
                            const name = rowCells(row)[1];
                            const typ = rowCells(row)[2];
                            if (path && (!showSnapshotEdges || rowCells(row)[5] === 'yes')) { // current
                                return p.h('a', {href: `/browse${path}${name}`}, p.h('code', {}, t + (typ === 'DIRECTORY' ? '/' : '')));
                            } else {
                                return p.h('a', {href: `/browse?id=${id}&name=${t}`}, p.h('code', {}, t + (typ === 'DIRECTORY' ? '/' : '')));
                            }
                        }
                    },
                },
                {name: 'Type', render: t => p.h('code', {}, t)},
                {name: 'Target', render: t => p.h('code', {}, t)},
                {name: 'CreationTime', render: t => p.h('code', {}, t)},
                ...(showSnapshotEdges ? [{name: 'Current', render: renderBool}]: []),
                ...(showSnapshotEdges ? [{name: 'Owned', render: renderBool}] : []),
                {name: 'Locked', render: renderBool},
            ];
            edgesTable = p.h(Table, {
                identifier: 'edges',
                rows,
                cols,
                elementsPerPage: 100,
                filterFromSearchParams: true,
                // the key forces rerender when the number of cols changes
                key: `edges-${showSnapshotEdges}`,
                paginateExtra: [p.h('div', {className: 'ms-2 form-check form-check-inline'},
                    p.h(
                        'input',
                        {
                            type: 'checkbox',
                            className: 'form-check-input',
                            checked: !!showSnapshotEdges,
                            onChange: (ev) => {
                                ev.preventDefault();
                                setState(prev => ({...prev, showSnapshotEdges: !prev.showSnapshotEdges}));
                            },
                        }
                    ),
                    p.h('label', {className: 'form-check-label'}, 'Show snapshot edges'),
                )],
            });
        }

        return edgesTable
    }

    const showSnapshotEdges = localStorage.getItem('edges-full');
    p.render(p.h(Edges, {id, path, showSnapshotEdges}), document.getElementById('edges-table'));
}

// transient files
// --------------------------------------------------------------------

export function renderTransientFiles() {
    function TransientFiles() {
        const [{shard, transientFiles, loadedTransientFiles}, setState] = useState({
            shard: 0,
            transientFiles: null,
            loadedTransientFiles: 0,
        });

        useEffect(async () => {
            const results = [];
            const req = {};
            while (true) {
                const resp = await shardReq('VISIT_TRANSIENT_FILES', shard, req);
                results.push(...resp.Files);
                if (resp.NextId === NULL_INODE_ID) { break; }
                req.BeginId = resp.NextId;
                setState(prev => ({...prev, loadedTransientFiles: results.length}));
            }
            setState(prev => ({...prev, transientFiles: results}));
        }, [shard]);

        const rows = [];
        if (transientFiles !== null) {
            for (const {Id, Cookie, DeadlineTime} of transientFiles) {
                rows.push([Id, Cookie, DeadlineTime]);
            }
        }
        const cols = [
            {name: 'Id', render: t => p.h('a', {href: `/browse?id=${t}`}, p.h('code', {}, t))},
            {name: 'Cookie', render: t => p.h('code', {}, t)},
            {name: 'Deadline', render: t => p.h('code', {}, t)},
        ]

        return [
            p.h('div', {className: 'input-group', style: 'width: 10rem;'},
                p.h('div', {className: 'input-group-text'}, 'Shard'),
                p.h('input', {
                    type: 'number', min: '0', max: '255', step: '1', className: 'form-control', value: shard.toString(),
                    onChange: (ev) => {
                        const shard = ev.target.value;
                        setState(prev => ({...prev, shard: parseInt(shard), transientFiles: null}));
                    },
                }),
            ),
            // key for redrawing...
            transientFiles === null ? p.h('em', {}, `Loading (${loadedTransientFiles}/?)...`) : p.h(Table, {key: shard, rows, cols, identifier: 'transient'}),
        ]
    }
    p.render(p.h(TransientFiles), document.getElementById('transient-files-table'));
}
