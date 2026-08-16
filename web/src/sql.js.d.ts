declare module 'sql.js' {
  export interface SqlJsStatic {
    Database: typeof Database;
  }

  export interface Database {
    exec(sql: string): QueryExecResult[];
    run(sql: string, params?: SqlValue[]): void;
    prepare(sql: string): Statement;
    export(): Uint8Array;
    getRowsModified(): number;
    close(): void;
  }

  export interface DatabaseConstructor {
    new (data?: ArrayLike<number> | Buffer | Uint8Array): Database;
  }

  export interface QueryExecResult {
    columns: string[];
    values: SqlValue[][];
  }

  export interface Statement {
    bind(params?: SqlValue[]): boolean;
    step(): boolean;
    get(): SqlValue[];
    getAsObject(): Record<string, SqlValue>;
    free(): void;
  }

  export type SqlValue = number | string | Uint8Array | null;

  export interface InitSqlJsOptions {
    locateFile?: (filename: string) => string;
    wasmBinary?: ArrayLike<number>;
  }

  export default function initSqlJs(
    options?: InitSqlJsOptions
  ): Promise<SqlJsStatic>;

  export const Database: DatabaseConstructor;
}