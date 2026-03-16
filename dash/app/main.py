import os
import dash
from dash import dcc, html, dash_table
import dash_auth
import clickhouse_connect
import pandas as pd
import plotly.express as px
from dotenv import load_dotenv

load_dotenv()

# App setup
app = dash.Dash(__name__, suppress_callback_exceptions=True)
server = app.server

# Authentication
USERNAME = os.getenv('DASH_USERNAME', 'admin')
PASSWORD = os.getenv('DASH_PASSWORD', 'admin')

auth = dash_auth.BasicAuth(
    app,
    {USERNAME: PASSWORD}
)

# Clickhouse connection
CH_HOST = os.getenv('CLICKHOUSE_HOST', 'clickhouse')
CH_PORT = int(os.getenv('CLICKHOUSE_PORT', '8123'))
CH_USER = os.getenv('CLICKHOUSE_USER', 'default')
CH_PASSWORD = os.getenv('CLICKHOUSE_PASSWORD', '')

def get_db_client():
    return clickhouse_connect.get_client(
        host=CH_HOST, 
        port=CH_PORT, 
        username=CH_USER, 
        password=CH_PASSWORD,
        database='analytics'
    )

def get_table_data():
    try:
        client = get_db_client()
        query = "SELECT * FROM analytics_events ORDER BY timestamp DESC LIMIT 1000"
        df = client.query_df(query)
        return df
    except Exception as e:
        print(f"Error connecting to Clickhouse: {e}")
        return pd.DataFrame()

def get_pie_data():
    try:
        client = get_db_client()
        query = "SELECT entity_type, count() as quantity FROM analytics_events GROUP BY entity_type"
        df = client.query_df(query)
        return df
    except Exception as e:
        print(f"Error connecting to Clickhouse: {e}")
        return pd.DataFrame()

# Styling
SIDEBAR_STYLE = {
    "position": "fixed",
    "top": 0,
    "left": 0,
    "bottom": 0,
    "width": "16rem",
    "padding": "2rem 1rem",
    "backgroundColor": "#f8f9fa",
    "fontFamily": "sans-serif"
}

CONTENT_STYLE = {
    "marginLeft": "18rem",
    "padding": "2rem 1rem",
    "fontFamily": "sans-serif"
}

# Layout
sidebar = html.Div(
    [
        html.H2("Analytics", className="display-4"),
        html.Hr(),
        html.P("Dashboard Menu", className="lead"),
        html.Ul(
            [
                html.Li(dcc.Link("Analytics Datas", href="/")),
                html.Li(dcc.Link("Pie Chart", href="/pie-chart")),
            ],
            style={"listStyleType": "none", "padding": 0, "lineHeight": "2"}
        ),
    ],
    style=SIDEBAR_STYLE,
)

content = html.Div(id="page-content", style=CONTENT_STYLE)

app.layout = html.Div([dcc.Location(id="url"), sidebar, content])

# Callbacks
@app.callback(
    dash.Output("page-content", "children"),
    [dash.Input("url", "pathname")]
)
def render_page_content(pathname):
    if pathname == "/pie-chart":
        df = get_pie_data()
        if df.empty:
            return html.Div("No data available or failed to connect to ClickHouse.")
        
        # Ensure we have data for the pie chart
        if 'entity_type' not in df.columns or 'quantity' not in df.columns:
            return html.Div("Missing required columns for pie chart.")
            
        fig = px.pie(
            df, 
            values='quantity', 
            names='entity_type', 
            title='Quantity by Entity Type',
            hole=0.3
        )
        
        return html.Div([
            html.H3("Entity Type Distribution"),
            dcc.Graph(figure=fig)
        ])
        
    else:
        # Default route (Table)
        df = get_table_data()
        if df.empty:
            return html.Div("No data available or failed to connect to ClickHouse.")
        
        return html.Div([
            html.H3("Analytics Events (Latest 1000)"),
            dash_table.DataTable(
                id='table',
                columns=[{"name": i, "id": i} for i in df.columns],
                data=df.to_dict('records'),
                page_size=50,
                style_table={'overflowX': 'auto'},
                style_cell={
                    'textAlign': 'left',
                    'padding': '10px',
                    'minWidth': '100px',
                },
                style_header={
                    'backgroundColor': 'rgb(230, 230, 230)',
                    'fontWeight': 'bold'
                },
                filter_action="native",
                sort_action="native",
                sort_mode="multi"
            )
        ])

if __name__ == '__main__':
    app.run_server(debug=True, host='0.0.0.0')