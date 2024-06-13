/* description: Compiles timble (a.k.a. somewhat modified Bitcoin Script) to a golang vm based parser. */

%{
    var beautify = require('js-beautify').js_beautify;
    var base = require('./config.js').base;
    var ScriptStack = require('./script-stack.js');
    var BN = require('bn.js')
%}

%nonassoc OP_ELSE
%nonassoc OP_ENDIF
%nonassoc OP_ENDMONAD

%start script

%% /* language grammar */

script
    : nonterm EOF
        %{
            var js = beautify($1);
            var evaluate;
            var result;
            var s = new ScriptStack(yy.env);

            evaluate = new Function('stack', 'BN', js);
            result = evaluate(s, BN);
            // in case of no return value, use stack length as a proxy of result
            if (result === undefined) {
                if (s.length === 0) {
                    result = true
                } else {
                  result = !(s.peek().toString() === '0')
                }
            }
            return {
                value: result,
                s: s,
                code: js
            };
        %}
    ;

nonterm
    : opcode
    | nonterm opcode
        %{
            $$ = $1 + $2;
        %}
    ;

opcode
    : DATA
        %{
            var value;
            if ($1.indexOf('OP_') !== -1) {
                // These statements encrypt their value as decimal, so convert
                value = parseInt($1.substr('OP_'.length)).toString(base);
            } else if ($1.indexOf('0x') !== -1) {
                // Otherwise, conversion takes place anyway when you push
                value = $1.substr('0x'.length);
            } else {
                value = $1;
            }
            $$ = 'stack.push("' + value + '");';
        %}
    | OP_TERMINAL
        %{
            // console.log(this);
            $$ = 'return stack.' + $1  + '();'
        %}
    | OP_IF nonterm OP_ELSE nonterm OP_ENDIF
        %{
            $$ = 'if (stack.pop().cmp(new BN(0)) !== 0) {' + $nonterm1 + '} else {' + $nonterm2 + '}';
        %}
    | OP_IF nonterm OP_ENDIF
        %{
            $$ = 'if (stack.pop().cmp(new BN(0)) !== 0) {' + $nonterm + '}';
        %}
    | OP_IFEQ nonterm OP_ENDIFEQ
        %{
            $$ = 'var a = stack.pop(); var b = stack.peek(); if (b.cmp(new BN(a)) === 0) { stack.pop(); ' + $nonterm + '}';
        %}
    | OP_MONAD nonterm OP_ENDMONAD
        %{
            $$ = 'stack.OP_MONAD(' + nonterm + ')';
        %}
    | OP_RATEMARKET nonterm OP_ENDRATEMARKET
        %{
            $$ = 'stack.OP_RATEMARKET(' + nonterm + ')';
        %}
    | OP_NOTIF nonterm OP_ELSE nonterm OP_ENDIF
        %{
            $$ = 'if (stack.pop().eq(new BN(0))) {' + $nonterm1 + '} else {' + $nonterm2 + '}';
        %}
    | OP_NOTIF nonterm OP_ENDIF
        %{
            $$ = 'if (stack.pop().eq(new BN(0))) {' + $nonterm + '}';
        %}
    | OP_NOP
        %{
            $$ = '';
        %}
    | OP_RETURN_RESULT
        %{
            $$ = 'if (stack.length === 0) { return true; } else { return stack.peek(); }'
        %}
    | OP_FUNCTION
        %{
            $$ = 'stack.' + $1  + '();'
        %}
    ;
