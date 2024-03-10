import React, { useState, useEffect } from 'react';
import { Title } from 'react-admin';
import ReactECharts from 'echarts-for-react';
import * as echarts from 'echarts';

interface MyNode {
    fixed?: boolean;
    x?: number;
    y?: number;
    symbolSize: number;
    id: string;
}

interface MyEdge {
    source: string;
    target: string;
}

const Graph = () => {
    const [option, setOption] = useState({
        series: [
            {
                type: 'graph',
                layout: 'force',
                animation: false,
                data: [{
                    fixed: true,
                    x: 300, // Adjust based on component size
                    y: 300, // Adjust based on component size
                    symbolSize: 20,
                    id: '-1',
                }] as MyNode[],
                force: {
                    repulsion: 100,
                    edgeLength: 5,
                },
                edges: [] as MyEdge[],
            },
        ],
    });

    useEffect(() => {
        const interval = setInterval(() => {
            setOption((prevOption) => {
                const newData = [...prevOption.series[0].data, { id: `${prevOption.series[0].data.length}`, symbolSize: 20 }] as MyNode[];
                const newEdges = [...prevOption.series[0].edges] as MyEdge[];
                const source = Math.round((newData.length - 1) * Math.random()).toString();
                const target = Math.round((newData.length - 1) * Math.random()).toString();
                if (source !== target) {
                    newEdges.push({ source, target });
                }
                return {
                    series: [
                        {
                            ...prevOption.series[0],
                            data: newData,
                            edges: newEdges,
                        },
                    ],
                };
            });
        }, 2000);

        return () => clearInterval(interval);
    }, []);

    return (
        <div>
            <Title title="Graph" />
            <ReactECharts option={option} style={{ height: 400, width: '100%' }} />
        </div>
    );
};

export default Graph;

