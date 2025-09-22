export interface MaskingRule {
    name: string;
    pattern: RegExp;
    replacement: string | ((match: string, ...groups: string[]) => string);
}
export declare class SensitiveDataDetector {
    private rules;
    constructor();
    private getDefaultRules;
    mask(text: string): [string, Array<{
        entityType: string;
        masked: string;
        count: number;
    }>];
    addRule(rule: MaskingRule): void;
    removeRule(name: string): void;
    getRules(): MaskingRule[];
}
//# sourceMappingURL=detectors.d.ts.map